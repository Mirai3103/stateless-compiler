sudo nsjail \
  --config nsjail-base.cfg \
  --bindmount_ro=$(pwd):/app \
  --bindmount_ro=/bin/bash
  --stats_file=./nsjail.stats \
  --cwd /app \
 -- /bin/bash -c "node a.js"
