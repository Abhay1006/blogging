# Lumina - Premium Blogging Platform

Lumina is a sleek, modern, and high-performance blogging platform built with a focus on rich typography and a premium user experience. It features a robust Go backend and a responsive React frontend, providing a seamless storytelling environment.



## 🚀 Key Features

- **Premium UI/UX**: Designed with a focus on clarity and aesthetics using Material UI and Apple-inspired design tokens.
- **Admin Dashboard**: Secure admin area for composing and publishing rich-text stories.
- **Rich Content Rendering**: Full Markdown support including LaTeX math equations and raw HTML rendering.
- **Real-time Interactions**: Like and dislike system with live count updates on the home page and post details.
- **Community Engagement**: Nested comment system for readers to share thoughts.
- **Secure Authentication**: JWT-based authentication with protected routes for administrative actions.

## 🛠 Tech Stack

### Frontend
- **React 19** with **TypeScript**
- **Vite** for ultra-fast builds
- **Material UI** for premium components
- **TanStack Query (React Query)** for efficient state management
- **React Markdown** & **KaTeX** for beautiful content rendering

### Backend
- **Go 1.26**
- **Fiber v2** (Express-inspired web framework)
- **SQLite** (via `modernc.org/sqlite`, pure Go -- no cgo) stored as a single local file
- **JWT** for secure authentication
- **Bcrypt** for password hashing

## ⚙️ Setup & Installation

### Prerequisites
- Go 1.26+
- Node.js 20+

No database server is required. The backend creates its SQLite file on first run.

### Backend Setup
1. Navigate to the `backend` directory:
   ```bash
   cd backend
   ```
2. Create a `.env` file based on the environment variables section below.
3. Install dependencies and run:
   ```bash
   go run main.go
   ```

### Frontend Setup
1. Navigate to the `frontend` directory:
   ```bash
   cd frontend
   ```
2. Install dependencies:
   ```bash
   npm install
   ```
3. Create a `.env` file with `VITE_API_URL=http://localhost:3000/api`.
4. Run the development server:
   ```bash
   npm run dev
   ```

## 🔒 Environment Variables

### Backend (.env)
See `backend/.env.example`.
- `DB_PATH`: Path to the SQLite file (default: `lumina.db`, created if missing).
- `STATIC_DIR`: Directory holding the built frontend. If absent, the server runs API-only.
- `PORT`: Server port (default: 3000).
- `JWT_SECRET`: Signing key for auth tokens. **Required in production** -- generate with `openssl rand -hex 32`.
- `ADMIN_EMAIL`: The email address that will be granted admin privileges upon signup.
- `ALLOWED_ORIGINS`: CORS whitelist. Only needed for the Vite dev server; production is same-origin.

### Frontend (.env)
- `VITE_API_URL`: The base URL for the backend API.

## 🗄 Data Model

The API is served under `/api`, which keeps it from colliding with the
client-side routes (`/login`, `/signup`) once the frontend is served from the
same origin in production.

Likes and dislikes live in a single `reactions` table keyed on
`(blog_id, user_id)`. That composite primary key is what makes a like and a
dislike from the same reader mutually exclusive -- a rule the previous MongoDB
schema had to enforce by hand with paired `$pull`/`$addToSet` updates.

### Migrating from MongoDB

`backend/cmd/migrate` copies an existing Atlas database into the local SQLite
file. Run it from a machine that can reach Atlas:

```bash
cd backend
go run ./cmd/migrate -mongo-uri "<atlas-uri>" -out lumina.db
```

Original document ids are preserved, so existing post links keep working.
Re-running is safe: every write is an upsert keyed on the original id.

## 🚀 Deployment

The target is a Termux/Android device reached over a Cloudflare tunnel.
`./deploy.sh` builds everything locally and ships it:

```bash
./deploy.sh                      # defaults: HOST=ssh.oniichad.com, PORT=3500
HOST=other.example ./deploy.sh
```

On the device, `~/lumina/` holds the binary, the `public/` frontend build,
`lumina.db`, and `start.sh` / `stop.sh` / `status.sh`.

### Why the build is the shape it is

Three Android-specific constraints shaped this, each found the hard way:

1. **`-buildmode=pie` is required.** A default `CGO_ENABLED=0` build produces a
   static `ET_EXEC` binary, which Android refuses with
   `unexpected e_type: 2`. Only position-independent `ET_DYN` is accepted.
2. **The ELF needs patching.** Go emits a TLS segment aligned to 8; ARM64
   Bionic demands at least 64, and otherwise fails with
   `TLS segment is underaligned`. `deploy.sh` builds and runs
   [termux-elf-cleaner](https://github.com/termux/termux-elf-cleaner) to fix
   that and to strip unsupported `DF_1_*` flags.
3. **Nothing is built on the device.** Go is installed there and building
   natively does work, but the toolchain's memory use trips Android's
   low-memory killer, which kills `sshd` and `cloudflared` and takes the
   device off the network mid-deploy.

### Ports

The device already serves other apps on 3000 and 4000, so Lumina defaults to
**3500**. Override with `PORT=... ./deploy.sh`.

### Process supervision

`start.sh` takes a `termux-wake-lock` and starts the server, plus a tunnel if
`~/lumina/.cloudflared-token` exists. It matches processes narrowly -- by
binary path, and by a metrics port unique to the blog's tunnel -- because a
broad `pkill cloudflared` would also kill the tunnel that serves SSH into the
device.

Nothing supervises these processes across reboots or OOM kills. `termux-boot`
is the fix if you want them to come back automatically.

## 📝 License
This project is for demonstration purposes. Built for stories.
