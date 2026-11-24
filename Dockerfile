# Runtime image for the deploy job. It does NOT compile the app: it only packages
# the binary that the build job already built and uploaded as an artifact.
#
# Build context = the downloaded artifact folder:
#   cybermedico   statically linked Go binary (static files, migrations and policies are embedded)
#   compose.sql   seed data, loaded when CYBERMEDICO_SEED_FILE points to it
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY cybermedico /app/cybermedico
COPY compose.sql /app/resources/config/cybermedico/compose.sql

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/cybermedico"]
