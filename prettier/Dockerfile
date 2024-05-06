FROM node:lts-alpine3.19
ENV HOME /home/user
RUN set -eux; \
	adduser -u 1001 -D -h "$HOME" user; \
	mkdir "$HOME/app"; \
	chown -R user:user "$HOME"
USER user
COPY ./package.* "$HOME/app/"
RUN cd "$HOME/app" && npm install --production
COPY ./*.js /app/
ENTRYPOINT ["$HOME/app/server.js"]
