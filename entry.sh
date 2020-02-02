#!/usr/bin/env bash

modprobe g_ffs && mkdir -p /dev/usb-ffs/bdb && mount -t functionfs bdb /dev/usb-ffs/bdb
exec ./bdbd /dev/usb-ffs/bdb
