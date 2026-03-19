#!/bin/sh
# Inject Umami analytics script into static HTML files at container startup.
# If UMAMI_WEBSITE_ID is not set, the app runs without tracking.

if [ -n "$UMAMI_WEBSITE_ID" ]; then
  SCRIPT_TAG="<script defer src=\"https://umami.dengshu.ovh/script.js\" data-website-id=\"$UMAMI_WEBSITE_ID\"></script>"
  for f in /app/static/*.html; do
    if ! grep -q "umami.dengshu.ovh" "$f" 2>/dev/null; then
      sed -i "s|</head>|  ${SCRIPT_TAG}\n</head>|" "$f"
    fi
  done
fi

exec "$@"
