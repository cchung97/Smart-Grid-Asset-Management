# Local build and deployment

The app is built and run on one machine with Docker Compose. There is no CI/CD pipeline and no cloud
target (see `ASSUMPTIONS.md`): tests are run by hand and "deploying" means running the commands below.

## Prerequisites

- Docker with Compose v2 (`docker compose version`).
- Only for running tests outside Docker: Go (the version in `backend/go.mod`) and Node 22+.
- Free ports: `8081` (app), `8080` (API and Swagger), `5432` (Postgres). All three are set in `.env`.

## First run

```bash
cp .env.example .env
# put your own key in API_KEY (no quotes or $ in it, nginx writes it into a config file)
openssl rand -hex 32
docker compose up --build        # add -d to run in the background
```

Compose starts, in order: `db` (Postgres) → `migrate` (applies `db/migrations/`, then exits) → `backend` →
`frontend` (nginx). Each waits for the previous one to be healthy, so the first start takes a minute.

| What | URL |
|---|---|
| The app | http://localhost:8081 |
| API docs (Swagger), also linked in the app's footer | http://localhost:8081/swagger/index.html, or http://localhost:8080/swagger/index.html |
| Health check | http://localhost:8081/healthz (`{"status":"ok","db":"up"}`) |

The API itself needs the `x-api-key` header. The app adds it for you (nginx), and so does the Swagger page when
opened on port 8081; on port 8080 press **Authorize** and enter the `API_KEY` from `.env`.

## Everyday commands

```bash
docker compose ps                          # state and health of each service
docker compose up -d                       # start everything in the background
docker compose stop                        # stop, keep containers and data
docker compose down                        # remove containers, keep the database volume
docker compose down -v                     # remove containers AND the database volume (all data lost)
docker compose up --build -d               # rebuild changed images and (re)start
docker compose up --build -d --force-recreate frontend   # rebuild and restart one service
```

Restart policy: `db`, `backend` and `frontend` use `restart: unless-stopped`, so they come back after a Docker
or machine restart unless you ran `docker compose stop`/`down`.

## Updating to a new version

```bash
git pull
docker compose up --build -d
```

Migrations run on every `up`. A change to `.env` needs `docker compose up -d --force-recreate` to take effect.
The app version shown in the footer is `version` in `frontend/package.json`.

## Running the tests

```bash
# Backend; database tests skip themselves unless TEST_DATABASE_URL is set
cd backend && go vet ./... && go test -race ./...

# Backend including the database tests (uses an isolated schema per test; your data is untouched)
docker compose up -d db
cd backend && TEST_DATABASE_URL='postgres://grid:grid@localhost:5432/grid_assets?sslmode=disable' go test -race ./...
#   use the POSTGRES_USER / POSTGRES_PASSWORD / POSTGRES_PORT / POSTGRES_DB values from .env

# Frontend
cd frontend && npm ci && npm run lint && npm run check:types && npm test && npm run build
```

If you change an endpoint or DTO: `cd backend && make docs`, then `cd frontend && npm run generate:types`.

## Troubleshooting

| Symptom | Check / fix |
|---|---|
| A service is not `healthy` | `docker compose ps`, then `docker compose logs --tail 100 <db\|migrate\|backend\|frontend>` |
| Watch a service live | `docker compose logs -f backend` (every line has a `tid=` trace id; the same id is in the `X-Trace-Id` response header) |
| Exact health state | `docker inspect --format '{{.State.Health.Status}}' smart-grid-asset-management-backend-1` |
| "port is already allocated" / address in use | `lsof -nP -iTCP:8081 -sTCP:LISTEN` shows what holds it. Stop it, or change `FRONTEND_PORT` / `BACKEND_PORT` / `POSTGRES_PORT` in `.env` and `docker compose up -d --force-recreate` |
| Every `/api` call returns 403 | `API_KEY` is empty in `.env`; set it and recreate: `docker compose up -d --force-recreate backend frontend` |
| Every `/api` call returns 401 | The key sent does not match. Through the app this means the backend and frontend were started with different keys: recreate both as above. When calling port 8080 yourself, send `-H "x-api-key: <API_KEY>"` |
| `migrate` fails, or the schema looks stale (for example after pulling a changed initial migration) | `docker compose down -v` then `docker compose up --build`. **This deletes all data** |
| Look inside the database | `docker compose exec db sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'`; for example `select count(*) from assets;` and `select version, dirty from schema_migrations;` |
| Nginx cannot reach the backend (502) | The backend is down or restarting: `docker compose ps backend`, then its logs |
| The app shows old code after a change | `docker compose up --build -d frontend`, then hard-refresh the browser |
| The page loads but the footer link to the API docs 404s | The frontend image is older than the `/swagger/` proxy: `docker compose up --build -d frontend` |
| Out of disk space / stale images | `docker image prune`; `docker system prune` removes all unused containers, networks and images (not volumes) |
| Start over completely | `docker compose down -v --rmi local` then `docker compose up --build` (deletes data and the built images) |
