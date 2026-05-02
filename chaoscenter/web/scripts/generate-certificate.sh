
mkdir -p certificates
openssl req -x509 -newkey rsa:4096 -keyout certificates/localhost-key.pem -out certificates/localhost.pem -days 365 -nodes -subj '/C=US/ST=CA/L=San Francisco/O=Local/CN=localhost'
