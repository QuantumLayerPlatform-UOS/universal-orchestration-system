#!/bin/bash
# Clear agents from MongoDB to resolve duplicate key issues

echo "Clearing agents from MongoDB..."
docker exec qlp-uos-mongodb-1 mongosh --quiet --eval "
use agent_manager;
db.agents.deleteMany({});
print('Agents cleared: ' + db.agents.countDocuments());
"

echo "Done. You can now restart the services."