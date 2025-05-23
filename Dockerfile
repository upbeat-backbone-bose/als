FROM node:lts-alpine AS builder_node_js_cache
ADD ui/package.json /app/package.json
WORKDIR /app
RUN npm i

FROM node:lts-alpine AS builder_node_js
ADD ui /app
WORKDIR /app
COPY --from=builder_node_js_cache /app/node_modules /app/node_modules
RUN npm run build \
    && chmod -R 650 /app/dist

FROM alpine:3 AS builder_golang
ADD backend /app
WORKDIR /app
COPY --from=builder_node_js /app/dist /app/embed/ui
RUN apk add --no-cache go 

RUN go mod tidy
RUN go build -o als && \
    chmod +x als

FROM alpine:3 AS builder_env
WORKDIR /app
ADD scripts /app
RUN sh /app/install-software.sh
RUN apk add --no-cache \
    iperf iperf3 \
    mtr \
    traceroute \
    iputils
RUN rm -rf /app

FROM alpine:3
LABEL maintainer="samlm0 <update@ifdream.net>"
COPY --from=builder_env / /
COPY --from=builder_golang --chmod=777 /app/als/als /bin/als

CMD ["als"]
