import express from 'express';
import { createServer } from 'http';
import { Server } from 'socket.io';

const app = express();
const server = createServer(app);
const io = new Server(server, {
  cors: { origin: '*' }
});

const PORT = process.env.PORT || 3001;

app.get('/health', (_req, res) => {
  res.json({ status: 'ok', service: 'forge-beacon' });
});

io.on('connection', (socket) => {
  console.log(`client connected: ${socket.id}`);
  socket.on('disconnect', () => {
    console.log(`client disconnected: ${socket.id}`);
  });
});

server.listen(PORT, () => {
  console.log(`forge-beacon listening on port ${PORT}`);
});
