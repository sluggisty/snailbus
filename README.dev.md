# Snailbus Development Setup

This guide explains how to use the development Docker Compose setup.

## Quick Start

The development setup automatically creates an admin user for testing:

```bash
docker compose -f docker-compose.dev.yml up --build
```

This will:
1. Start PostgreSQL database
2. Run migrations
3. Create an admin user with:
   - Username: `admin`
   - Password: `change me`
   - Email: `admin@localhost`
4. Generate an API key for the admin user
5. Start the Snailbus API server

## Admin User Details

**Default Credentials:**
- Username: `admin`
- Password: `change me`
- Email: `admin@localhost`

You can customize these by setting environment variables:
- `ADMIN_USERNAME` - Admin username (default: `admin`)
- `ADMIN_PASSWORD` - Admin password (default: `change me`)
- `ADMIN_EMAIL` - Admin email (default: `admin@localhost`)

## Getting Your API Key

After the containers start, check the logs for the `create-admin` service:

```bash
docker compose -f docker-compose.dev.yml logs create-admin
```

You'll see output like:
```
Admin user 'admin' created successfully with ID: <uuid>
Admin API key created: <your-api-key>
You can use this API key to authenticate: X-API-Key: <your-api-key>
```

## Using the API Key

Once you have the API key, you can use it to authenticate:

### Via curl:
```bash
curl -H "X-API-Key: <your-api-key>" http://localhost:8080/api/v1/hosts
```

### Via Login Endpoint:
You can also login via the web UI or API:
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"change me"}'
```

This will return a new API key in the response.

## Services

- **snailbus**: Main API server (port 8080)
- **postgres**: PostgreSQL database (exposed on port 5433 to avoid conflicts with local PostgreSQL)
- **create-admin**: One-time service that creates the admin user

**Note:** If you have a local PostgreSQL instance running on port 5432, the dev setup uses port 5433 to avoid conflicts. To connect to the dev database from outside Docker, use `localhost:5433`.

## Stopping Services

```bash
docker compose -f docker-compose.dev.yml down
```

To also remove volumes (this will delete all data):
```bash
docker compose -f docker-compose.dev.yml down -v
```

## Differences from Production

The dev setup:
- Uses `GIN_MODE=debug` for verbose logging
- Exposes PostgreSQL port 5432 for debugging
- Automatically creates an admin user
- Uses separate volume (`postgres_data_dev`) to avoid conflicts

