# Lumina - Premium Blogging Platform

Lumina is a sleek, modern, and high-performance blogging platform built with a focus on rich typography and a premium user experience. It features a robust Go backend and a responsive React frontend, providing a seamless storytelling environment.

![Lumina Header](frontend/src/assets/hero.png)

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
3. Create a `.env` file with `VITE_API_URL=http://localhost:3000`.
4. Run the development server:
   ```bash
   npm run dev
   ```

## 🔒 Environment Variables

### Backend (.env)
- `MONGO_URI`: Your MongoDB connection string.
- `PORT`: Server port (default: 3000).
- `ADMIN_EMAIL`: The email address that will be granted admin privileges upon signup.
- `ALLOWED_ORIGINS`: CORS whitelist (e.g., `http://localhost:5173`).

### Frontend (.env)
- `VITE_API_URL`: The base URL for the backend API.

## 📝 License
This project is for demonstration purposes. Built for stories.
