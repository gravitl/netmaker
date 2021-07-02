#!/bin/bash
set -e

while true; do
	./netclient checkin;
	sleep 30
done
