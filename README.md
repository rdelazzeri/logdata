# Logging Data Service
This is docker service to collect log messages send from another docker services in the same network.
Made in Go with Grok3.0 help.

## Request Exemple
curl.exe -X POST http://localhost:8015/logdata/ -H "Authorization: Bearer secret123" -H "Content-Type: application/json" -d '{
    "account": "cont123",
    "system": "sys456",
    "user": "user789",
    "module": "auth",
    "task": "task101",
    "timestamp": "2025-07-19T12:00:00Z",
    "msg": "User logged in",
    "level": 30
}'
