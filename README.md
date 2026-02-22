# Server Manager

A real-time process and service manager for monitoring and controlling game servers, web services, and databases on Windows.

## Features

- **Real-time monitoring**: CPU, memory, thread count, uptime
- **Start/Stop controls**: Executables and Windows Services
- **Auto-restart**: Automatic process recovery on crash
- **Metrics history**: CPU and memory graphs (1m–60m windows)
- **Live logs**: Real-time log viewer with search/filter
- **Bulk operations**: Start/stop all processes
- **Event timeline**: Track start/stop/crash events
- **Dark mode**: Light/dark theme toggle
- **In-app config**: Edit process configuration without restarting

## Tech Stack

- **Backend**: Go 1.21+ (port 8090)
- **Frontend**: React 19 + Vite 7 (port 5173)
- **Database**: MySQL (optional, for monitored services)

## Setup

### Prerequisites

- **Go 1.21+** — [Download](https://golang.org/dl/)
- **Node.js 18+** — [Download](https://nodejs.org/)
- **Git**

### Configuration

1. Copy `backend/config.json` and replace placeholders with your paths:
   - `YOUR_GO_EXECUTABLE_PATH` → Path to `go.exe` (e.g., `C:\Program Files\Go\bin\go.exe`)
   - `YOUR_WEBSERVER_WORKING_DIR` → Your web server project directory
   - `YOUR_MYSQL_SERVICE_NAME` → Windows Service name for MySQL (if used)
   - `YOUR_AUTHSERVER_EXECUTABLE_PATH` → Path to authserver executable
   - `YOUR_AUTHSERVER_WORKING_DIR` → Authserver working directory
   - `YOUR_WORLDSERVER_EXECUTABLE_PATH` → Path to worldserver executable
   - `YOUR_WORLDSERVER_WORKING_DIR` → Worldserver working directory

2. Place `config.json` in the `backend/` directory

### Backend Build & Run

```bash
cd backend
go mod download
go build -o server-manager.exe
```

Run the backend (requires Administrator):
```bash
./server-manager.exe
```

The backend will start on `http://localhost:8090`

### Frontend Build & Run

```bash
cd frontend
npm install
npm run dev
```

The frontend will start on `http://localhost:5173` with proxy to backend

### Production Build

Frontend:
```bash
cd frontend
npm run build
```

Output: `frontend/dist/` — serve with any static host

## Usage

1. Start backend (Administrator required for Windows Service control)
2. Start frontend dev server or serve built frontend
3. Open `http://localhost:5173` (or your frontend URL)
4. Configure process paths in the in-app config editor
5. Use controls to start/stop processes and monitor in real-time

## API Routes

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/processes` | List all processes and status |
| POST | `/api/processes/{id}/start` | Start a process |
| POST | `/api/processes/{id}/stop` | Stop a process |
| PUT | `/api/processes/{id}/autorestart` | Toggle auto-restart |
| GET | `/api/processes/{id}/logs` | Fetch process logs |
| GET | `/api/processes/{id}/metrics` | Historical metrics |
| GET | `/api/config` | Fetch configuration |
| PUT | `/api/config` | Update configuration |
| GET | `/api/events` | Event timeline |
| GET | `/ws` | WebSocket endpoint (real-time updates) |

## Architecture

- **Backend (`backend/`)**: Go HTTP server with WebSocket support
  - `main.go` — Server setup and routing
  - `config.go` — Configuration loading
  - `process.go` — Process/service management
  - `handlers.go` — API endpoint handlers
  - `ws.go` — WebSocket connections
  - `metrics.go` — Metrics storage (1-hour history)
  - `events.go` — Event timeline storage

- **Frontend (`frontend/`)**: React + Vite
  - `App.jsx` — Main app layout
  - `components/` — React components
  - `services/api.js` — WebSocket client

## Notes

- Backend must run as **Administrator** for Windows Service control
- Processes are configured in `config.json` — paths must point to valid executables or service names
- Log files are stored in the backend working directory
- WebSocket updates every 1 second
- Metrics retained for 1 hour (3600 samples)

## License

[Add your license here]
