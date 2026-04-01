#!/bin/bash
set -e

systemctl stop do-obsd || true
systemctl disable do-obsd || true
systemctl stop do-otelcol || true
systemctl disable do-otelcol || true
