# Do I need to install a Linux OS first?
# Get go compiler from https://hub.docker.com/_/golang/
FROM golang:1.6

MAINTAINER jason@obroin.net

# Copy source code into the image
COPY . /go/src/decryptDiags

# Build
RUN cd /go/src/decryptDiags && go build

# Change permissions
RUN chmod 777 /go/src/decryptDiags

#Set the working directory
WORKDIR /go/src/decryptDiags

# Run the app
#ENTRYPOINT /go/src/decryptDiags -w
CMD ["/go/src/decryptDiags/decryptDiags", "-w"]

# Make a persistent volume for uploaded diags
VOLUME ["/go/src/decryptDiags/uploads"]

# Expose port 8000
EXPOSE 8000

# Alternative Dockerfile
# Use minimal Linux image
#FROM alpine
#COPY . /go/src/decryptDiags
# Change permissions
#RUN chmod 777 /go/src/decryptDiags-lx
#WORKDIR /go/src/decryptDiags
#provide access to a location for diag files (i.e. where upload will be mounted)
#...
#CMD ["decryptDiags-lx", "-w"]
#EXPOSE 8000
