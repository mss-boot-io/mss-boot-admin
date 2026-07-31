
## Frontend startup commands
**IMPORTANT**: Frontend startup commands must be run asynchronously to avoid blocking the terminal.

### Prerequisites
- Node.js v22+ must be available (nvm installation at ~/.nvm/versions/node/v22.21.1)
- pnpm must be in PATH

### Correct async startup pattern
```bash
# Set Node.js PATH first:
export PATH="$HOME/.nvm/versions/node/v22.21.1/bin:$PATH"

# From mss-boot-admin-antd directory, use setsid to run in background:
cd /home/lwx/go/src/github.com/mss-boot-io/mss-boot-admin-antd
setsid pnpm dev > /tmp/frontend.log 2>&1 &
sleep 10
lsof -i :8000  # Verify port is listening

# Check logs:
tail -30 /tmp/frontend.log
```

### WRONG - will block terminal
```bash
# These will cause the terminal to hang:
pnpm dev           # Blocks until killed
npm run dev        # Blocks until killed
```

### Proxy configuration
- Frontend proxies `/admin/` -> `http://localhost:8080`
- All backend APIs are at `/admin/api/*`
- Requires backend running on port 8080 first

### Full stack startup sequence
```bash
# 1. Start Redis (if not running):
docker start redis || docker run -d --name redis -p 6379:6379 redis

# 2. Start backend:
cd /home/lwx/go/src/github.com/mss-boot-io/mss-boot-admin
go build -o /tmp/mss-boot-admin .
setsid /tmp/mss-boot-admin server > /tmp/backend.log 2>&1 &
sleep 5

# 3. Start frontend:
export PATH="$HOME/.nvm/versions/node/v22.21.1/bin:$PATH"
cd /home/lwx/go/src/github.com/mss-boot-io/mss-boot-admin-antd
setsid pnpm dev > /tmp/frontend.log 2>&1 &
sleep 10

# 4. Verify:
lsof -i :8080  # Backend
lsof -i :8000  # Frontend
curl http://localhost:8000/  # Should return HTML
curl http://localhost:8000/admin/api/options  # Should return 401 (needs auth)
```
