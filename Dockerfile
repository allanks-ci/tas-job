FROM scratch
EXPOSE 8080

WORKDIR /server
ADD static /server/static
ADD main /server/tas-job

ENTRYPOINT ["./tas-job"]
CMD [""]