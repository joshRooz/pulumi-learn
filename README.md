# Pulumi Learn

Follows the Pulumi Learn path through [Pulumi Fundamentals](https://www.pulumi.com/learn/pulumi-fundamentals/) and [Building with Pulumi](https://www.pulumi.com/learn/building-with-pulumi/).

Port definitions removed from backend and mongo containers to avoid `pulumi up` port conflicts with multiple stacks. Since the containers can talk to each other through the docker network, only the frontend port needs to be exposed to the OS. After that, the `dev` and `staging` stacks played nice together.

```sh
# dataSeedContainer is removed automatically in the resource definition
# but pulumi state doesn't appear to know. manually remove it from
# state to cleanly destroy
stack="dev"
pulumi state delete \
  urn:pulumi:${stack}::fundamentals::docker:index/container:Container::dataSeedContainer \
  -y -s "${stack}" && \
  pulumi destroy -s "${stack}" -y

# remove app images
docker rmi "backend:${stack}" "frontend:${stack}"
```