FROM mongo:latest
ADD ./replicate.js /replicate.js
ADD ./setup.sh /setup.sh
RUN chmod +x /setup.sh
ENTRYPOINT ["/setup.sh"]
