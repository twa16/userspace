#!/bin/bash
service ssh start
tail -f /var/log/dmesg > /dev/null