FROM node:25 AS builder

WORKDIR /build

COPY web/package*.json ./
RUN npm ci

COPY web .
RUN npm run build

FROM node:25-alpine

WORKDIR /app

RUN npm install -g servor

COPY --from=builder /build/dist ./

EXPOSE 3000

CMD ["servor", ".", "index.html", "3000"]
