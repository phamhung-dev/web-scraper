![alt](https://go.dev/images/go-logo-blue.svg)

# WEB SCRAPPER

![alt](https://img.shields.io/badge/go%20version-%3D%201.21.3-brightgreen) ![alt](https://img.shields.io/badge/platform-linux%20%7C%20windows-lightgrey) ![alt](https://img.shields.io/badge/build-passing-brightgreen) ![alt](https://img.shields.io/badge/coverage-100%25-brightgreen) [![Facebook Badge](https://img.shields.io/badge/facebook-%40phamhung-blue)](https://www.facebook.com/kunn.ngoc.5/) [![LinkedIn Badge](https://img.shields.io/badge/linkedin-%40phamhung2503-blue)](https://www.linkedin.com/in/phamhung2503/) [![Github Bagde](https://img.shields.io/badge/github-%40phamhung250301-blue)](https://github.com/pham-hung-25-03-01)

<b>Note</b>: This project is only for learning purpose.

This is a web scrapper project using Golang. It uses chromedp to get data from the website and elastic search to store data.

### Deployment
If you want to run this project on docker, please follow the below steps:
1. docker compose up -d web-scraper-db
2. docker exec web-scraper-db sed -i '/^network:/{n;s/host:.*/host: 0.0.0.0/;}' /opt/bitnami/elasticsearch/config/elasticsearch.yml
3. docker compose up -d web-scraper-server