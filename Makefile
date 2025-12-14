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
	$(MAKE) -j2 run-backend run-frontend
