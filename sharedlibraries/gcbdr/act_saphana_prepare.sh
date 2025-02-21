#!/bin/bash
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
DBSID=$1
userstorekey=$2
HANAVERSION=$3


tdate=`date +%Y%m%d%H%M%S`
BASEPATH="/etc/google-cloud-sap-agent/gcbdr"
FILELOC="$BASEPATH/touch"
if [ ! -d "$FILELOC" ]; then
   mkdir -p $FILELOC
   chmod 755 $FILELOC
else
   chmod 755 $FILELOC
fi
DBSID=`echo $DBSID | tr '[a-z]' '[A-Z]'`
DBADM=`echo $DBSID | tr '[A-Z]' '[a-z]'`
DBADM=$DBADM'adm'
dbuser=$DBADM
if [ ! -d $FILELOC/FAIL_MSG ]; then
   mkdir -p $FILELOC/FAIL_MSG
   chown "$DBADM":sapsys $FILELOC/FAIL_MSG
else
   rm -f $FILELOC/FAIL_MSG/*
   chown "$DBADM":sapsys $FILELOC/FAIL_MSG
fi

FAIL_MSG=$FILELOC/FAIL_MSG
source $BASEPATH/act_saphana_comm_func.sh
globalpath=`su - $dbuser -c 'echo $DIR_INSTANCE'`
globalpath=`dirname $globalpath`

SSLENFORCE=`grep "^sslenforce" $globalpath/SYS/global/hdb/custom/config/global.ini`
SSLENFORCE=`echo $SSLENFORCE | awk -F "=" '{print $2}'|xargs`
SSLENFORCE=`echo $SSLENFORCE | tr '[A-Z]' '[a-z]'`

if [ "$SSLENFORCE" = "true" ]; then
   SECUDIR=`su - $dbuser -c 'echo $SECUDIR'`
   hdbsql="hdbsql -e -sslprovider commoncrypto -sslkeystore $SECUDIR/sapsrv.pse -ssltruststore $SECUDIR/sapsrv.pse"
else
   hdbsql="hdbsql"
fi

####################################### Check DBSID ###################################
if [ ! -z $DBSID ]; then
   backup_checkdbsid "$DBSID"
   if [ "$?" -gt 0 ]; then
      echo "ERRORMSG: Backup Pre Check: The specified Database SID \"$DBSID\" no longer exists!"
      exit 1
   fi
fi

####################################### Check usestore key ###################################
if [ ! -z "$userstorekey" ]; then
   backup_check_userstorekey "$userstorekey" "$dbuser"
   if [ "$?" -gt 0 ]; then
      echo "ERRORMSG: Backup Pre Check: hdbuserstore key $userstorekey does not exist!"
      exit 1
   fi
fi

####################################### Check connection ###################################
INSTANCENUM=`su - $dbuser -c 'env | grep TINSTANCE= | cut -d"=" -f2'`
if [ -z "$INSTANCENUM" ]; then
   INSTANCENUM=`su - $dbuser -c 'basename $DIR_INSTANCE | rev | cut -c 1-2 | rev'`
fi

if [ -z "$INSTANCENUM" ]; then
   echo "ERRORMSG: Backup Pre Check: Unable to determine the instance number from HANA environment variables (TINSTANCE/DIR_INSTANCE)."
   exit 1
fi

HVERSIONTMP=`echo $HANAVERSION |sed 's/\.//g'`
if [ "$HVERSIONTMP" -lt "200000" ]; then
   PORTNO=3"$INSTANCENUM"15
else
   PORTNO=3"$INSTANCENUM"13
fi
if [ ! -z "$PORTNO" ]; then
   backup_check_connection "$userstorekey" "$DBSID" "$PORTNO" "$FAIL_MSG"
fi

####################################### Check Privs ###################################

backup_check_privs "$userstorekey"

####################################### Check if save point exists###################################

backup_check_snapshot "$userstorekey"

####################################### Check backint ###################################

backup_backint_check "$userstorekey"

####################################### Check DATA, LOG, LOGBACKUPATH FS Usage ############################

DATAPATH=`cat $globalpath/SYS/global/hdb/custom/config/global.ini | grep "^basepath_datavolumes" | awk -F"=" '{print $2}' |xargs`
LOGPATH=`cat $globalpath/SYS/global/hdb/custom/config/global.ini | grep "^basepath_logvolumes" | awk -F"=" '{print $2}' |xargs`
LOGBKPPATH=`cat $globalpath/SYS/global/hdb/custom/config/global.ini | grep "^basepath_logbackup" | awk -F"=" '{print $2}' |xargs`

backup_check_fs_usage "$DATAPATH" "$LOGPATH" "$LOGBKPPATH"

####################################### Check tenant status ############################
backup_check_tenant_status "$userstorekey"

####################################### Check license type ############################
exit 0
