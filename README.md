# Lumina - Premium Blogging Platform

Lumina is a sleek, modern, and high-performance blogging platform built with a focus on rich typography and a premium user experience. It features a robust Go backend and a responsive React frontend, providing a seamless storytelling environment.



## 🚀 Key Features

- **Premium UI/UX**: Designed with a focus on clarity and aesthetics using Material UI and Apple-inspired design tokens.
- **Admin Dashboard**: Secure admin area for composing and publishing rich-text stories.
- **Rich Content Rendering**: Full Markdown support including LaTeX math equations and raw HTML rendering.
- **Interactions**: Like and dislike posts; counts are shown on the home page and post details.
- **Community Engagement**: Comments for readers to share thoughts.
- **Secure Authentication**: JWT-based authentication with protected routes for administrative actions.
- **Safe Rendering**: Raw HTML in posts is sanitized before display.

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
- **MongoDB** (Atlas) for flexible data storage
- **JWT** for secure authentication
- **Bcrypt** for password hashing

## ⚙️ Setup & Installation

### Prerequisites
- Go 1.26+
- Node.js 20+
- MongoDB Atlas account (or local MongoDB)

### Backend Setup
1. Navigate to the `backend` directory:
   ```bash
   cd backend
   ```
2. Copy `.env.example` to `.env` and fill it in (at minimum, set `JWT_SECRET`).
3. Install dependencies and run:
   ```bash
   go run .
   ```
4. Run the tests:
   ```bash
   go test ./...
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
3. Copy `.env.example` to `.env` (it points at `http://localhost:10000`).
4. Run the development server:
   ```bash
   npm run dev
   ```
5. Lint and test:
   ```bash
   npm run lint
   npm test
   ```

## 🔒 Environment Variables

### Backend (.env)
- `JWT_SECRET` (**required**): Secret used to sign login tokens. Use a random string of at least 32 characters (e.g. `openssl rand -base64 48`). The server will not start without it.
- `MONGO_URI`: Your MongoDB connection string (default: `mongodb://localhost:27017`).
- `DB_NAME`: Database name (default: `lumina_db`).
- `PORT`: Server port (default: `10000`).
- `ADMIN_EMAIL`: The email address that will be granted admin privileges upon signup. Leave unset for no admin.
- `ALLOWED_ORIGINS`: Comma-separated CORS whitelist (default: `http://localhost:5173,http://127.0.0.1:5173`).

### Frontend (.env)
- `VITE_API_URL`: The base URL for the backend API (default: `http://localhost:10000`).

## 📝 License
This project is for demonstration purposes. Built for stories.
