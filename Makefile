run-backend:
	cd backend && PORT=4000 go run .

run-frontend:
	cd frontend && flutter run -d chrome --web-hostname=0.0.0.0 --web-port=8080 --release

run:
	$(MAKE) -j2 run-backend run-frontend
