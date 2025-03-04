#!/bin/sh
# Start mosquitto
pkill -9 mosquitto
cp -R -u -p /app/mosquitto_config /data
mosquitto -d -c /data/mosquitto_config/mosquitto.conf

# Create logs directory if not exists
mkdir -p /data/logs

# Start supervisor and nginx
/usr/bin/supervisord &
nginx -g "daemon off;"
