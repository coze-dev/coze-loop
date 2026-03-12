/**
 * Mock Server Entry Point
 * Code Evaluator API Mock Server
 */

import express, { Application, Request, Response, NextFunction } from 'express';
import cors from 'cors';
import bodyParser from 'body-parser';
import apiRoutes from './api';

const app: Application = express();
const PORT = process.env.PORT || 9999;

// Middleware
app.use(cors());
app.use(bodyParser.json());
app.use(bodyParser.urlencoded({ extended: true }));

// Request logging middleware
app.use((req: Request, res: Response, next: NextFunction) => {
	console.log(`[${new Date().toISOString()}] ${req.method} ${req.path}`);
	next();
});

// Mount API routes
app.use(apiRoutes);

// Health check endpoint
app.get('/health', (req: Request, res: Response) => {
	res.json({
		status: 'ok',
		timestamp: new Date().toISOString(),
		service: 'Mock Server is running',
	});
});

// 404 handler
app.use((req: Request, res: Response) => {
	res.status(404).json({
		error: 'Not Found',
		message: `Route ${req.method} ${req.path} not found`,
	});
});

// Error handler
app.use((err: Error, req: Request, res: Response, next: NextFunction) => {
	console.error('Error:', err);
	res.status(500).json({
		error: 'Internal Server Error',
		message: err.message,
	});
});

// Start server
app.listen(PORT, () => {
	console.log('='.repeat(50));
	console.log('Code Evaluator Mock Server');
	console.log('='.repeat(50));
	console.log(`Server is running on port ${PORT}`);
	console.log(`Health check: http://localhost:${PORT}/health`);
	console.log(`API base: http://localhost:${PORT}/api/evaluation/v1`);
	console.log('='.repeat(50));
});

export default app;
