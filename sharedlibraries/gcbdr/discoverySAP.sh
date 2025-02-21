#!/bin/bash
#!/bin/sh
# =============================================================================
# Copyright 2024 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# =============================================================================

set -x
BASEPATH="/etc/google-cloud-sap-agent/gcbdr/"
XMLFILE="/etc/google-cloud-sap-agent/gcbdr/SAPHANA.xml"
SCRIPTS="/etc/google-cloud-sap-agent/gcbdr/backup/CustomApp_SAPHANA.sh"
FTYPE="SAPHANA"
DBUSER=""
DBSID=""
KEYNAME=""
INSTANCENUM=""
HVERSION=""
VERSION=""
DATAVOL=""
DATAPATH=""
LOGVOL=""
LOGPATH=""
DATAVOLUMEOWNER=""
PYTHONPATH=""


get_lgname()
{
  mntpt=$1
  #LGNAME=`grep -w "$mntpt" /proc/mounts | awk '{print $1}'`
  LGNAME=`df "$mntpt" | tail -1 | awk '{ print $1 }'`
  echo $LGNAME
}

get_vgname()
{
  lgname=$1
  VGNAME=`lvdisplay $lgname | grep "VG Name" | awk '{print $3}'`
  echo $VGNAME
}

get_lvname()
{
  lgname=$1
  LVNAME=`lvdisplay $lgname | grep "LV Name" | awk '{print $3}'`
  echo $LVNAME
}


get_pvname()
{
  vgname=$1
  lvname=$2
  #PVNAME=`lvdisplay -v $vgname |grep "PV Name" | awk '{print $3}' |sed -z 's/\n/,/'`
  #PVNAME=`lvs --noheading -o devices /dev/$vgname/$lvname |awk -F"(" '{print $1}'|uniq|sed -z 's/\n/,/'`
  PVNAME=`vgdisplay -v $vgname |grep -w "PV Name" | awk '{print $3}'|uniq|sed -z 's/\n/,/'`
  echo $PVNAME
}

check_pddisk()
{
  pvname=$1
  IS_PD=
  if [[ "$pvname" =~ "nvme" ]]; then
     IS_PD="$(nvme id-ns -b $pvname | xxd -p -seek 384 -l 256 | xxd -p -r | awk -F":" '{print $3}' |awk -F"}" '{print $1}')"
     IS_PD="$(echo $IS_PD |sed 's/"//g')"
     if [[ "$IS_PD" == "PERSISTENT" ]]; then
        IS_PD=1
     fi
  else
     IS_PD=`udevadm info $pvname | grep "scsi-0Google_PersistentDisk" | grep -v "grep" | wc -l`
  fi
  if [ "$IS_PD" -gt 0 ]; then
     echo "TRUE"
  else
     echo "FALSE"
  fi
}

get_pdname()
{
  pvname=$1
  if [[ "$pvname" =~ "nvme" ]]; then
     pdname="$(nvme id-ns -b $pvname | xxd -p -seek 384 -l 256 | xxd -p -r |awk -F":" '{print $2}' |awk -F"," '{print $1}')"
     pdname="$(echo $pdname |sed 's/"//g')"
  else
      pdname=`udevadm info $pvname | grep "scsi-0Google_PersistentDisk" | grep "S:" |awk -F"/" '{print $3}' |sed 's/scsi-0Google_PersistentDisk_//g' | awk -F"-part" '{print $1}'`
  fi
  echo $pdname
}

writetoxml()
{
   tag=$1
   mountpoint=$2
   pv=$3
   vgname=$4
   lvname=$5
   pdname=$6
   echo -e "\t\t\t<pd mounttag=\"$tag\" mountpoint=\"$mountpoint\" pddisk=\"$pv\" vgname=\"$vgname\" lvname=\"$lvname\" pddiskname=\"$pdname\" > " >> $XMLFILE
   echo -e "\t\t\t</pd>" >> $XMLFILE
}

generate_pd_details()
{
   TAG=$1
   MOUNT_PNT=$2

   LGVOL=
   VGNAME=
   LVNAME=
   PVNAMES=
   IS_PD=
   PD_NAME=

   LGVOL=`get_lgname $MOUNT_PNT`
   VGNAME=`get_vgname $LGVOL`
   if [ ! -z "$VGNAME" ]; then
      LVNAME=`get_lvname $LGVOL`
      PVNAMES=`get_pvname "$VGNAME" "$LVNAME"`
   else
      PVNAMES=$LGVOL
   fi
   if [[ ! -z $MOUNT_PNT ]]; then
      MOUNT_OPTIONS="$(grep -w $MOUNT_PNT /proc/mounts|grep -v etc |awk '{print $4}')"
   fi
   echo -e "\t\t\t<volume name=\"$TAG\" mountpoint=\"$MOUNT_PNT\" vgname=\"$VGNAME\" lvname=\"$LVNAME\" mountoptions=\"$MOUNT_OPTIONS\" >" >> $XMLFILE
   echo -e "\t\t\t\t<pddisks>" >> $XMLFILE
   for pvs in $(echo $PVNAMES |tr ',' ' ')
   do
     IS_PD=`check_pddisk $pvs`
     if [ "$IS_PD" = "TRUE" ]; then
        PD_NAME=`get_pdname $pvs`
        #writetoxml "$TAG" "$MOUNT_PNT" "$pvs" "$VGNAME" "$LVNAME" "$PD_NAME"
        echo -e "\t\t\t\t\t<pd disk=\"$pvs\" devicename=\"$PD_NAME\" />" >> $XMLFILE
     fi
   done
   echo -e "\t\t\t\t</pddisks>" >> $XMLFILE
   echo -e "\t\t\t</volume>" >>  $XMLFILE
}

remove_file()
{
  fname=$1
  if [ -f "$fname" ]; then
     rm -f $fname
  fi
}

#################################### MAIN ################################

if [[ ! -d "$BASEPATH" ]]; then
   mkdir -p $BASEPATH
   mkdir -p $BASEPATH/touch
fi

timeout=1
while [ $timeout -lt 10 ]
do
  if [ -f $BASEPATH/touch/globaliniupdate ]; then
     sleep 10
  else
     break;
  fi
  timeout=$(($timeout+1))
done

echo -e "<applications>" > $XMLFILE
#getting SAPHANA dbuser
ps -ef | grep -i hdbnameserver | grep -v grep | awk '{print $1}' | while read line
do
DBUSER=$line

if [ -z "$DBUSER" ]; then
   echo "failed to find any running instance"
#   exit 1
fi

#get dbsid
DBSID=`echo $DBUSER | cut -c 1-3`

INSTANCENUM=`su - $DBUSER -c 'env | grep TINSTANCE= | cut -d"=" -f2'`

if [ -z "$INSTANCENUM" ]; then
   INSTANCENUM=`su - $DBUSER -c 'basename $DIR_INSTANCE | rev | cut -c 1-2 | rev'`
fi

HVERSION=`su - $DBUSER -c "HDB version | grep -i version: | cut -d':' -f2 | sed -e 's/^[ \t]*//'"`
VERSION_CHK=`echo $HVERSION | cut -c1-3`

if [ "$VERSION_CHK" = "1.0" ]; then
   VERSION=$VERSION_CHK
else
   VERSION=`echo $HVERSION | cut -c1-8`
fi

#globalinipath=`locate global.ini | grep -i "custom/config/global.ini" | grep -i $DBSID`

globalpath=`su - $DBUSER -c 'echo $DIR_INSTANCE'`
globalpath=`dirname $globalpath`
globalinipath=$globalpath/SYS/global/hdb/custom/config/global.ini
configpath=$globalpath/SYS/global/hdb/custom/config

DATAVOL=`grep -iw ^basepath_datavolumes $globalinipath | cut -d"=" -f2 | sed -e 's/^[ \t]*//'`
LOGVOL=`grep -iw ^basepath_logvolumes $globalinipath | cut -d"=" -f2 | sed -e 's/^[ \t]*//'`
LOGBACKUPPATH=`grep -iw ^basepath_logbackup $globalinipath | cut -d"=" -f2 | sed -e 's/^[ \t]*//'|sort|uniq`
CATLOGBACKUPPATH=`grep -iw ^basepath_catalogbackup $globalinipath | cut -d"=" -f2 | sed -e 's/^[ \t]*//'|sort|uniq`
CATLOGBACKUPPATH_UNQCNT=`grep -iw ^basepath_catalogbackup $globalinipath|uniq | wc -l`
CATLOGBACKUPPATH_CNT=`grep -iw ^basepath_catalogbackup $globalinipath | wc -l`

DATAVOL_MNT_EXISTS=`df -P $DATAVOL |tail -1 | wc -l`
LOGVOL_MNT_EXISTS=`df -P $LOGVOL |tail -1 | wc -l`

if [ "$DATAVOL_MNT_EXISTS" -eq 0 ] && [ "$LOGVOL_MNT_EXISTS" -eq 0 ]; then
   echo "ERRORMSG: No mounted volumes found for DATA/LOG. Please check!"
   continue
elif [ "$DATAVOL_MNT_EXISTS" -eq 0 ]; then
     echo "ERRORMSG: No mounted volumes found for DATA. Please check!"
     continue
elif [ "$LOGVOL_MNT_EXISTS" -eq 0 ]; then
     echo "ERRORMSG: No mounted volumes found for LOG. Please check!"
     continue
fi

if [ ! -d "$LOGBACKUPPATH" ]; then
   TMPLOGBACKUPPATH=`dirname $LOGBACKUPPATH`
   if [ ! -d "$TMPLOGBACKUPPATH" ]; then
      echo "WARNINGMSG: Log Backup Path $LOGBACKUPPATH does not exist for $DBSID! Please check!"
      continue
   fi
fi

if [ ! -z "$LOGBACKUPPATH" ] && [ "$CATLOGBACKUPPATH_UNQCNT" -gt 1 ]; then
    sed -i "/basepath_catalogbackup/d" $globalinipath
    sed -i "/\bpersistence\b/a basepath_catalogbackup = $LOGBACKUPPATH" $globalinipath
elif [ ! -z "$LOGBACKUPPATH" ] && [ -z "$CATLOGBACKUPPATH" ]; then
    sed -i "/\bpersistence\b/a basepath_catalogbackup = $LOGBACKUPPATH" $globalinipath
fi

CATLOGBACKUPPATH=`grep -iw ^basepath_catalogbackup $globalinipath | cut -d"=" -f2 | sed -e 's/^[ \t]*//'`
LOGMODE=`grep -i ^log_mode $globalinipath | cut -d"=" -f2 | sed -e 's/^[ \t]*//'`

HANANODE=`cat $configpath/nameserver.ini | grep -w "^worker" | awk -F "=" '{print $2}'`
MASTERNODE=`cat $configpath/nameserver.ini | grep -w "^active_master" | awk -F "=" '{print $2}' | awk -F ":" '{print $1}'`
STANDBYNODE=`cat $configpath/nameserver.ini | grep -w "^standby" | awk -F "=" '{print $2}' | awk -F ":" '{print $1}'`
SITENAME=`cat $configpath/global.ini | grep -w "^site_name" | awk -F "=" '{print $2}' | awk -F ":" '{print $1}'`
HARDWAREKEY=`cat $configpath/nameserver.ini | grep -w "^id" | awk -F "=" '{print $2}' | awk -F ":" '{print $1}'`

dbuser=`echo $DBSID | tr '[A-Z]' '[a-z]'`
dbuser="$dbuser"adm

if [ ! -d $BASEPATH/touch ]; then
   mkdir -p $BASEPATH/touch
   chmod 755 $BASEPATH/touch
else
   chmod 755 $BASEPATH/touch
fi

HANANODE_COUNT=`echo $HANANODE |wc -w`
if [ "$HANANODE_COUNT" -ge 1 ] && [ ! -z "$STANDBYNODE" ]; then
   CLUSTERTYPE="scaleout"
elif [ "$HANANODE_COUNT" -gt 1 ]; then
     CLUSTERTYPE="scaleout"
fi

if [ "$CLUSTERTYPE" != "scaleout" ]; then
   CHECK_REPLICATION_FILE=$BASEPATH/touch/replica_$DBSID.txt
   remove_file $CHECK_REPLICATION_FILE
   if [ ! -f $CHECK_REPLICATION_FILE ]; then
      touch $CHECK_REPLICATION_FILE
      chown $dbuser:sapsys $CHECK_REPLICATION_FILE
   fi
   REPLICATION_ENABLED=`su - $dbuser -c "hdbnsutil -sr_state>$CHECK_REPLICATION_FILE "`
   REPLICATION_ENABLED=`grep -w "operation mode:" $CHECK_REPLICATION_FILE |awk -F":" '{print $2}'|xargs`

   if [ "$REPLICATION_ENABLED" = "logreplay" ] || [ "$REPLICATION_ENABLED" = "logreplay_readaccess" ] || [ "$REPLICATION_ENABLED" = "sync" ] || [ "$REPLICATION_ENABLED" = "delta_datashipping" ]; then
      #MASTERNODE=`grep -w "primary masters" $CHECK_REPLICATION_FILE | awk -F":" '{print $2}'|xargs`
      REPLICATEDNODES=`cat $CHECK_REPLICATION_FILE | awk '/Site Mappings:/{f=0} f; /Host Mappings:/{f=2}' |awk -F"]" '{print $2}' |xargs`
      REPLICATEDNODES=`echo $REPLICATEDNODES |sed "s/$MASTERNODE//g"`
      CLUSTERTYPE="replication"
   elif [ "$REPLICATION_ENABLED" = "primary" ]; then
        MASTERNODE=`su - $DBUSER -c 'HDBSettings.sh landscapeHostConfiguration.py --sapcontrol=1 | grep -iw host  | awk -F"=" '"'"'{print $2}'"'"' '`
        REPLICATEDNODES=`cat $CHECK_REPLICATION_FILE| awk '/Site Mappings:/{f=0} f; /Host Mappings:/{f=2}' |awk -F"]" '{print $2}' |xargs`
        REPLICATEDNODES=`echo $REPLICATEDNODES |sed "s/\b$MASTERNODE\b$//g"`
        CLUSTERTYPE="replication"
   fi
   remove_file "$CHECK_REPLICATION_FILE"

   if [ ! -z "$REPLICATEDNODES" ]; then
       HANANODE=$MASTERNODE' '$REPLICATEDNODES
   fi

   EXTENDED_WORKER=`cat $configpath/nameserver.ini | grep -w "extended_storage_worker" |awk -F"=" '{print $1}' |awk -F"_" '{print $2}' |xargs`
   HANANODE=$HANANODE' '$STANDBYNODE
   HANANODE=$HANANODE' '$EXTENDED_WORKER

   if [ ! -z "$EXTENDED_WORKER" ]; then
      CLUSTERTYPE="dynamictiering"
   fi
fi
if [ -z "$CLUSTERTYPE" ]; then
   CONFIGTYPE="scaleup"
else
   CONFIGTYPE=$CLUSTERTYPE' cluster'
fi
SSLENFORCE=`grep "sslenforce" $globalpath/SYS/global/hdb/custom/config/global.ini`
SSLENFORCE=`echo $SSLENFORCE | awk -F "=" '{print $2}'|xargs`

if [ "$SSLENFORCE" = "true" ]; then
   hdbsql="hdbsql -e -sslprovider commoncrypto -sslkeystore $SECUDIR/sapsrv.pse -ssltruststore $SECUDIR/sapsrv.pse"
else
   hdbsql="hdbsql"
fi

### Added for MDL Calculation ####
if [ "$VERSION" = "1.0" ]; then
   MULTIDB=`grep "mode" $globalpath/SYS/global/hdb/custom/config/global.ini | grep -iw multidb |awk -F"=" '{print $2}'`
   MULTIDB=`echo $MULTIDB | tr '[A-Z]' '[a-z]'|xargs`
fi

if [ "$VERSION" = "1.0" ] && [ "$MULTIDB" != "multidb" ]; then
   MDLSQL="select sum(TOTAL_SIZE) from M_VOLUME_FILES where file_type='DATA'"
else
   MDLSQL="select sum(TOTAL_SIZE) from sys_databases.M_VOLUME_FILES where file_type='DATA'"
fi

localnode=`hostname`
if [[ ! -z $USERSTOREKEY ]]; then
   uuidsql="select top 1 FILE_NAME from sys_databases.M_DATA_VOLUMES where host='$localnode'"
   uuid=`su - $dbuser -c "$hdbsql -U $USERSTOREKEY -a -j -x \"$uuidsql\""`
   uuid=`dirname $uuid`
   uuid=`dirname $uuid`
   uuid=`basename $uuid`
fi

CHECKDATAMNT="$(cat /proc/mounts | grep $DATAVOL |awk '{print $2}')"
if [ ! -z "$CHECKDATAMNT" ]; then
  DATAPATH=$CHECKDATAMNT
else
  DATAPATH=${DATAVOL::-4}
fi
CHECKDATAPATH=`df -P $DATAPATH | awk 'NR==2{print $NF}'`
if [ -z "$CHECKDATAPATH" ]; then
  echo "ERRORMSG: Could not get the DATA mount path"
fi

CHECKLOGMNT="$(cat /proc/mounts | grep $LOGVOL |awk '{print $2}')"
if [ ! -z "$CHECKLOGMNT" ]; then
  LOGPATH=$CHECKLOGMNT
else
  LOGPATH=${LOGVOL::-4}
fi
CHECKLOGPATH=`df -P $LOGPATH | awk 'NR==2{print $NF}'`
if [ -z "$CHECKLOGPATH" ]; then
  echo "ERRORMSG: Could not get the LOG mount path"
fi

DATAVOLUMEOWNER=`ls -l $DATAPATH |grep -iw "$DBSID" |grep -v "grep" | awk '{print $3":"$4}'`

echo "DBUSER=$DBUSER DBSID=$DBSID  INSTANCENUM=$INSTANCENUM HVERSION=$HVERSION VERSION=$VERSION"
echo "DATAVOL=$DATAVOL LOGVOL=$LOGVOL"
echo "DATAPATH=$DATAPATH LOGPATH=$LOGPATH"
echo "LOGBACKUPPATH=$LOGBACKUPPATH"

PORT=$INSTANCENUM
DBPORT='HDB'$PORT

if [ ! -z "$USERSTOREKEY" ]; then
   SQL="select string_agg(database_name,',')  from m_databases"
   dbuser=`echo $DBSID | tr '[A-Z]' '[a-z]'`
   dbuser="$dbuser"adm
   dbnames=`su - $dbuser -c "$hdbsql -U $USERSTOREKEY -a -j -x \"$SQL\""`
   dbnames=`echo $dbnames | sed 's/"//g'`
   DBSIZE=`su - $dbuser -c "$hdbsql -U $USERSTOREKEY -a -j -x \"$MDLSQL\""`
   DBSIZE=`echo $DBSIZE | sed 's/"//g'`
   SQL="SELECT HARDWARE_KEY FROM M_LICENSE WHERE PRODUCT_NAME='SAP-HANA'"
   HARDWAREKEY=`su - $dbuser -c "$hdbsql -U $USERSTOREKEY -a -j -x \"$SQL\""`
   HARDWAREKEY=`echo $HARDWAREKEY | sed 's/"//g'`
   SQL="select distinct SITE_NAME  from PUBLIC.M_SYSTEM_REPLICATION"
   SITENAME=`su - $dbuser -c "$hdbsql -U $USERSTOREKEY -a -j -x \"$SQL\""`
   SITENAME=`echo $SITENAME | sed 's/"//g'`
fi

echo "*****************: data path: $DATAPATH ***********"

localnode=`hostname`
uuidcheck=`echo $DATAVOL |grep mnt |wc -l`
if [ "$uuidcheck" -gt 0 ]; then
   uuid=`basename $DATAVOL`
else
   uuid=`ls -l $DATAVOL | grep mnt | awk '{print $9}'`
fi

if [ "$CLUSTERTYPE" = "scaleout" ]; then
   STANDBYNODE=`su - $dbuser -c "HDBSettings.sh landscapeHostConfiguration.py --sapcontrol=1 | grep hostActualRoles | grep standby | awk -F'/' '{print \\$2}' |xargs"`
   HANANODE=`su - $dbuser -c "HDBSettings.sh landscapeHostConfiguration.py --sapcontrol=1 | grep hostActualRoles | grep worker | awk -F'/' '{print \\$2}' |xargs"`
fi
echo -e "\t<application name=\"$DBSID\" friendlytype=\"$FTYPE\" instance=\"$INSTANCENUM\" DBSID=\"$DBSID\" PORT=\"$INSTANCENUM\" DBPORT=\"$DBPORT\" version=\"$VERSION\" datavolowner=\"$DATAVOLUMEOWNER\" hananodes=\"$HANANODE\" masternode=\"$MASTERNODE\" standbynode=\"$STANDBYNODE\" extendedworker=\"$EXTENDED_WORKER\" keyname=\"$KEYNAME\" dbnames=\"$dbnames\" dbsize=\"$DBSIZE\" uuid=\"$uuid\" hardwarekey=\"$HARDWAREKEY\" sitename=\"$SITENAME\" configtype=\"$CONFIGTYPE\" clustertype=\"$CLUSTERTYPE\" replication_nodes=\"$REPLICATEDNODES\" >" >> $XMLFILE
echo -e "\t\t<files>" >> $XMLFILE
if [ -d $DATAPATH ]; then

############# GENERATE VOLUME for each  NODE ###################
   echo -e "\t\t\t<file path=\"$DATAPATH\" datavol=\"$DATAVOL\" >" >> $XMLFILE
   if [ ! -z "$USERSTOREKEY" ]; then
#      nodelist=`echo $HANANODE |sed "s/$STANDBYNODE//g"`
       nodelist=`su - $dbuser -c "HDBSettings.sh landscapeHostConfiguration.py --sapcontrol=1 | grep hostActualRoles | grep worker | awk -F'/' '{print \\$2}' |xargs"`
#       nodelist=`echo $nodelist |awk -F"/" '{print $2}' |xargs`
      for nodename in $(echo $nodelist)
      do
        sql="select top 1 FILE_NAME from sys_databases.M_DATA_VOLUMES where host='$nodename'"
        datapath=`su - $dbuser -c "$hdbsql -U $USERSTOREKEY -a -j -x \"$sql\""`
        datapath=`dirname $datapath`
        datapath=`dirname $datapath`
        datapath=`echo $datapath | sed 's/"//g'`
        echo -e "\t\t\t\t<cluster nodename=\"$nodename\" path=\"$datapath\" />" >>$XMLFILE
      done
   fi
   echo -e "\t\t\t</file>" >>$XMLFILE
fi
if [ -d $LOGPATH ]; then
   echo -e "\t\t\t<file path=\"$LOGPATH\" logvol=\"$LOGVOL\" >" >>$XMLFILE
   if [ ! -z "$USERSTOREKEY" ]; then
      # nodelist=`echo $HANANODE |sed "s/$STANDBYNODE//g"`
       nodelist=`su - $dbuser -c "HDBSettings.sh landscapeHostConfiguration.py --sapcontrol=1 | grep hostActualRoles | grep worker | awk -F'/' '{print \\$2}' |xargs"`
      for nodename in $(echo $nodelist)
      do
        sql="select top 1 FILE_NAME from sys_databases.m_log_segments where host='$nodename'"
        logpath=`su - $dbuser -c "$hdbsql -U $USERSTOREKEY -a -j -x \"$sql\""`
        logpath=`dirname $logpath`
        logpath=`dirname $logpath`
        logpath=`echo $logpath | sed 's/"//g'`
        echo -e "\t\t\t\t<cluster nodename=\"$nodename\" path= \"$logpath\" />" >>$XMLFILE
      done
   fi
   echo -e "\t\t\t</file>" >>$XMLFILE
fi
echo -e "\t\t</files>" >> $XMLFILE
############# GENERATE DATABASE MASTER NODE INFO ###################
if [ ! -z "$USERSTOREKEY" ]; then
   echo -e "\t\t<databases>" >> $XMLFILE
   sql=
   dbnameslist=`echo $dbnames | sed 's/SYSTEMDB//g'`
   for dbname in $(echo $dbnameslist | tr ',' ' ')
   do
     echo -e "\t\t\t<database name=\"$dbname\">" >> $XMLFILE
     sql="select distinct host from sys_databases.m_services where database_name='$dbname' and DETAIL='master'"
     masterdbnode=`su - $dbuser -c "$hdbsql -U $USERSTOREKEY -a -j -x \"$sql\""`
     masterdbnode=`echo $masterdbnode | sed 's/"//g'`
     sql="select string_agg(host,',')  from sys_databases.m_services where database_name='$dbname' and DETAIL<>'master' and SQL_PORT<>0"
     othernodes=`su - $dbuser -c "$hdbsql -U $USERSTOREKEY -a -j -x \"$sql\""`
     othernodes=`echo $othernodes | sed 's/"//g'`
     if [ ! -z "$masterdbnode" ]; then
        echo -e "\t\t\t\t<cluster nodename=\"$masterdbnode\" master=\"yes\" />" >> $XMLFILE
     fi
     if [ ! -z "$othernodes" ] && [ "$othernodes" != "?" ]; then
        for worker in $(echo $othernodes | tr ',' ' ' )
        do
          echo -e "\t\t\t\t<cluster nodename=\"$worker\" master=\"no\" />" >> $XMLFILE
        done
     fi
     echo -e "\t\t\t</database>" >> $XMLFILE
   done
   echo -e "\t\t</databases>" >> $XMLFILE
fi

############### Generate Cluster Process Info ####################
if [ ! -z "$USERSTOREKEY" ]; then
   echo -e "\t\t<clusternodes>" >> $XMLFILE
   node=
   for node in $(echo $nodelist)
   do
      echo -e "\t\t\t<cluster nodename=\"$node\">" >> $XMLFILE
      sql=
      sql="select COORDINATOR_TYPE from sys_Databases.m_services where host='$node' and service_name='indexserver'"
      indexservertype=`su - $dbuser -c "$hdbsql -U $USERSTOREKEY -a -j -x \"$sql\""`
      indexservertype=`echo $indexservertype | xargs`
      for idx in $(echo $indexservertype)
      do
        echo -e "\t\t\t\t<service name=\"indexserver\" type=\"$idx\" /> " >> $XMLFILE
      done
      sql="select service_name||'-'||DATABASE_NAME||'-'||PORT||'-'||SQL_PORT||'-'||COORDINATOR_TYPE from sys_databases.m_services where host='$node' and service_name not in ('indexserver')"
      servicenames=`su - $dbuser -c "$hdbsql -U $USERSTOREKEY -a -j -x \"$sql\""`
      servicenames=`echo $servicenames | xargs`
      for service in $(echo $servicenames)
      do
        servicename=`echo $service | awk -F"-" '{print $1}'`
        DBNAME=`echo $service | awk -F"-" '{print $2}'`
        PORT=`echo $service | awk -F"-" '{print $3}'`
        SQL_PORT=`echo $service | awk -F"-" '{print $4}'`
        COORDINATOR_TYPE=`echo $service | awk -F"-" '{print $5}'`
        echo -e "\t\t\t\t<service name=\"$servicename\" databasename=\"$DBNAME\" port=\"$PORT\" sqlport=\"$SQL_PORT\" coordinatortype=\"$COORDINATOR_TYPE\" />" >> $XMLFILE
      done
      echo -e "\t\t\t</cluster>" >> $XMLFILE
   done
   echo -e "\t\t</clusternodes>" >> $XMLFILE

fi
echo -e "\t\t<logbackuppath>" >> $XMLFILE
echo -e "\t\t\t<file path=\"$LOGBACKUPPATH\" />" >>$XMLFILE
echo -e "\t\t</logbackuppath>" >> $XMLFILE
echo -e "\t\t<globalinipath>" >> $XMLFILE
if [ -d $globalpath ]; then
   echo -e "\t\t\t<file path=\"$globalinipath\" />" >>$XMLFILE
fi
echo -e "\t\t</globalinipath>" >> $XMLFILE
echo -e "\t\t<catalogbackuppath>" >> $XMLFILE
if [ -d $CATLOGBACKUPPATH ]; then
   echo -e "\t\t\t<file path=\"$CATLOGBACKUPPATH\" />" >>$XMLFILE
fi
echo -e "\t\t</catalogbackuppath>" >> $XMLFILE
echo -e "\t\t<logmode " >> $XMLFILE
if [ ! -z $LOGMODE ]; then
   echo -e "\t\t\tmode=\"$LOGMODE\" " >>$XMLFILE
fi
echo -e "\t\t/>" >> $XMLFILE
echo -e "\t\t<scripts>" >> $XMLFILE
echo -e "\t\t\t<script phase=\"all\" path=\"$SCRIPTS\" />" >> $XMLFILE
echo -e "\t\t</scripts>" >> $XMLFILE
echo -e "\t\t<volumes>" >> $XMLFILE

if [ ! -z "$DATAPATH" ] && [ -d "$DATAPATH" ]; then
   DMNTPT=`df -P $DATAPATH | awk 'NR==2{print $NF}'`
   if [[ "$DMNTPT" != "/" ]]; then
       generate_pd_details "datavol" $DMNTPT
   fi
fi
if [ ! -z "$LOGPATH" ] && [ -d "$LOGPATH" ]; then
   LMNTPT=`df -P $LOGPATH | awk 'NR==2{print $NF}'`
   if [[ "$LMNTPT" != "/" ]]; then
      generate_pd_details "logvol" $LMNTPT
   fi
fi
if [ ! -z "$LOGBACKUPPATH" ] && [ -d "$LOGBACKUPPATH" ]; then
   LGBKPMNTPT=`df -P $LOGBACKUPPATH | awk 'NR==2{print $NF}'`
   if [[ "$LGBKPMNTPT" != "/" ]]; then
      generate_pd_details "logbackup" $LGBKPMNTPT
   fi
fi
DBSID=`echo $DBSID | tr '[a-z]' '[A-Z]'`
HANASHARED=`ls -l /usr/sap/$DBSID/HDB"$INSTANCENUM" | awk -F"->" '{print $2}'`
DATAMNT_LGVOL=`get_lgname $DMNTPT`
DATAMNT_VGNAME=`get_vgname $DATAMNT_LGVOL`
if [ ! -z "$HANASHARED" ]; then
   HANASHAREDMNTPT=`df -P $HANASHARED | awk 'NR==2{print $NF}'`
   HANASHARED_LGVOL=`get_lgname $HANASHAREDMNTPT`
   if [[ ! "$HANASHARED_LGVOL" =~ "/dev/sd" ]]; then
       HANASHARED_VGNAME=`get_vgname $HANASHARED_LGVOL`
   fi
   if [ "$HANASHARED_VGNAME" = "$DATAMNT_VGNAME" ]; then
      generate_pd_details "hanashared" "$HANASHAREDMNTPT"
   fi
fi
if [ -d "/usr/sap" ]; then
   USRSAPMNTPT="$(df -P /usr/sap  | awk 'NR==2{print $NF}')"
   if [[ "$USRSAPMNTPT" != "/" ]]; then
      USRSAP_LGVOL="$(get_lgname $USRSAPMNTPT)"
      if [[ ! "$USRSAP_LGVOL" =~ "/dev/sd" ]]; then
         USRSAP_VGNAME="$(get_vgname $USRSAP_LGVOL)"
      fi
      if [[ "$USRSAP_VGNAME" == "$DATAMNT_VGNAME" ]]; then
         generate_pd_details "usrsap" "$USRSAPMNTPT"
      fi
   fi
fi
   echo -e "\t\t</volumes>" >> $XMLFILE

echo -e "\t</application>" >>$XMLFILE
#echo -e "</applications>" >> $XMLFILE
#echo -e "" >> $XMLFILE


DBADM=`echo $DBSID | tr '[A-Z]' '[a-z]'`
DBADM=$DBADM'adm'

PORT=$INSTANCENUM
DBPORT='HDB'$PORT

done
echo -e "</applications>" >> $XMLFILE
echo -e "" >> $XMLFILE
exit 0
