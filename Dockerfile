FROM golang:1.21.3

ENV http_proxy socks5h://10.2.94.95:5555
ENV https_proxy socks5h://10.2.94.95:5555
ENV socks_proxy socks5h://10.2.94.95:5555
ENV no_proxy localhost,127.0.0.0/8,::1,web-scraper-db

RUN echo 'Acquire::http::proxy "socks5h://10.2.94.95:5555";' | tee -a /etc/apt/apt.conf.d/proxy.conf && \
    echo 'Acquire::https::proxy "socks5h://10.2.94.95:5555";' | tee -a /etc/apt/apt.conf.d/proxy.conf && \
    echo 'Acquire::socks::proxy "socks5h://10.2.94.95:5555";' | tee -a /etc/apt/apt.conf.d/proxy.conf

RUN apt-get update && \
    apt-get -y upgrade  && \
    apt-get install -y curl && \
    apt-get install -y dpkg && \
    apt-get install -y xdg-utils libxrandr2 libxkbcommon0 libxfixes3 libxext6 libxdamage1 libxcomposite1 libxcb1 libx11-6 && \
    apt-get install -y libvulkan1 libu2f-udev libpango-1.0-0 libnss3 libnspr4 libgtk-4-1 libglib2.0-0 libgbm1 libdrm2 && \
    apt-get install -y libdbus-1-3 libcups2 libcairo2 libatspi2.0-0 libatk1.0-0 libatk-bridge2.0-0 libasound2 fonts-liberation

RUN curl -LO https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb && \
    dpkg -i google-chrome-stable_current_amd64.deb && \
    rm -rf google-chrome-stable_current_amd64.deb

ENV http_proxy socks5://10.2.94.95:5555
ENV https_proxy socks5://10.2.94.95:5555
ENV socks_proxy socks5://10.2.94.95:5555

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY ./ ./

CMD ["go", "run", "cmd/main.go" ]