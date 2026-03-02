FROM alpine

# Libraries needed for go binaries
RUN apk add libc6-compat

COPY build/powershelf_manager /opt/powershelf_manager/powershelf_manager

EXPOSE 50051

CMD /opt/powershelf_manager/powershelf_manager serve
