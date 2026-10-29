FROM nginx:1.25-alpine

COPY dist /usr/share/nginx/html
# 静态资源必须对 nginx worker 可读：源文件权限若被带成 0600，容器内会直接 403。
RUN chmod -R a+rX /usr/share/nginx/html
# 说明：本文件是「以 frontend/ 为构建上下文」的独立变体，nginx 配置与
# deploy/nginx.conf 是两份拷贝，改动必须同步两边（生产用 deploy/Dockerfile.frontend，
# 那里直接 COPY deploy/nginx.conf，没有这份重复）。
COPY <<'EOF' /etc/nginx/conf.d/default.conf
# WebSocket 升级用：/api/v1/reviews/{id}/progress 是 WS 端点，
# 不带这两个头代理会剥掉 Upgrade，hertz 握手失败返回 400。
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;

    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;

    location / {
        try_files $uri $uri/ /index.html;
        # SPA 入口不缓存：协商缓存拿最新 index.html，防止发版后浏览器用旧入口引用已删除的旧 chunk。
        add_header Cache-Control "no-cache";
    }

    # 带内容哈希的静态资源：长缓存。
    location /assets/ {
        add_header Cache-Control "public, immutable, max-age=31536000";
    }

    location /api/ {
        proxy_pass http://review-service:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        # 必须是 $http_host（含端口），不能是 $host（nginx 会剥掉端口）：
        # 进度 WebSocket 的同源校验拿 Origin 的 host:port 与 Host 头严格比对，
        # 端口被剥掉后带端口的访问会被判跨站而 403，实时进度流断掉。
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }

    location ^~ /swagger/ {
        proxy_pass http://review-service:8080;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /openapi.yaml {
        proxy_pass http://review-service:8080;
        proxy_set_header Host $http_host;
    }

    location /healthz {
        proxy_pass http://review-service:8080;
    }

    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
EOF

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
