1. run db

docker run -d \
  --name customflow-postgres \
  -p 5432:5432 \
  -e POSTGRES_DB=customflow \
  -e POSTGRES_USER=celentris \
  -e POSTGRES_PASSWORD=Thara2224 \
  -v postgres_data:/var/lib/postgresql/data \
  postgres:15



2. update flyway conf with db credentials then

flyway migrate



3. then run app

docker run -d \
  --name customflow-backend \
  --restart unless-stopped \
  -p 8080:8080 \
  -e PORT=8080 \
  -e DB_HOST=16.171.147.121 \
  -e DB_PORT=5432 \
  -e DB_USER=celentris \
  -e DB_PASSWORD=Thara2224 \
  -e DB_NAME=customflow \
  -e OPENAI_API_KEY=your_openai_api_key_here \
  -e GIN_MODE=release \
  -v $(pwd)/uploads:/root/uploads \
  sathishkumarnce/customflow-backend:latest


  docker run -d \
  --name customflow-frontend \
  --restart unless-stopped \
  -p 3000:3000 \
  sathishkumarnce/customflow-frontend:latest