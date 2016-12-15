FROM scratch
EXPOSE 8080

ADD tas-job /tas-job

ENTRYPOINT ["/tas-job"]
CMD [""]