FROM scratch
EXPOSE 8080

WORKDIR /server
COPY static /server/static
COPY main /server/tas-job

ENTRYPOINT ["./tas-job"]
CMD [""]