.PHONY: up down logs sample-upload

up:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f

sample-upload:
	curl -s -X POST -F "file=@sample-data/emails.json" http://localhost:8080/api/emails/upload | jq .
