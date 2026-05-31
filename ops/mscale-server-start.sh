#!/bin/bash
# PM2 startup wrapper — loads Google OAuth secrets then runs the API binary.
set -a
source /home/ubuntu/mscale-server/google_oauth.env
set +a
exec /home/ubuntu/mscale-server/mscale-server
