#!/bin/sh /etc/rc.common
#Created by oycol<oycol527@outlook.com>

EXTRA_COMMANDS="status"
EXTRA_HELP="        status      Check service is running"
START=99

LOG_FILE="/tmp/netclient.logs"

start() {
  if [ ! -f "${LOG_FILE}" ];then
      touch "${LOG_FILE}"
  fi
  local PID=$(ps -e|grep "netclient checkin -n all"|grep -v grep|awk '{print $1}')
  if [ "${PID}" ];then
    echo "service is running"
    return
  fi
  bash -c "while [ 1 ]; do /etc/netclient/netclient checkin -n all >> ${LOG_FILE} 2>&1;sleep 15;\
           if [ $(ls -l ${LOG_FILE}|awk '{print $5}') -gt 10240000 ];then tar zcf "${LOG_FILE}.tar" -C / "tmp/netclient.logs"  && > $LOG_FILE;fi;done &"
  echo "start"
}

stop() {
  local PID=$(ps -e|grep "netclient checkin -n all"|grep -v grep|awk '{print $1}')
  if [ "${PID}" ];then
    kill "${PID}"
  fi
  echo "stop"
}

status() {
  local PID=$(ps -e|grep "netclient checkin -n all"|grep -v grep|awk '{print $1}')
  if [ "${PID}" ];then
    echo -e "netclient[${PID}] is running \n"
  else
    echo -e "netclient is not running \n"
  fi
}