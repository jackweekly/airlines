install:
	mkdir -p backend/data
	curl -L -o backend/data/airports.csv https://ourairports.com/data/airports.csv
	cd backend && go mod tidy
	cd frontend && flutter pub get

run-backend:
	cd backend && PORT=4000 go run .

run-frontend:
	cd frontend && flutter run -d chrome --web-hostname=0.0.0.0 --web-port=8080 --release

run:
	@set -e; \
	trap 'kill $${BACK_PID} 2>/dev/null' EXIT INT TERM; \
	(cd backend && PORT=4000 go run .) & \
	BACK_PID=$$!; \
	echo "backend started (pid=$${BACK_PID})"; \
	(cd frontend && flutter run -d chrome --web-hostname=0.0.0.0 --web-port=8080 --release); \
	kill $${BACK_PID} 2>/dev/null || true; \
	wait $${BACK_PID} 2>/dev/null || true
