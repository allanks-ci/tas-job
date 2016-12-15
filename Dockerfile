FROM scratch
EXPOSE 8080

ADD main /tas-job

ENTRYPOINT ["/tas-job"]
CMD [""]