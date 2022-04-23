import * as pulumi from "@pulumi/pulumi";
import * as docker from "@pulumi/docker";

// get configuration
const config = new pulumi.Config();
const frontendPort = config.requireNumber("frontend_port");
const backendPort = config.requireNumber("backend_port");
const mongoPort = config.requireNumber("mongo_port");
const mongoHost = config.require("mongo_host");
const database = config.require("database");
const nodeEnvironment = config.require("node_environment");
const mongoUsername = config.require("mongo_username");
export const mongoPassword = config.requireSecret("mongo_password");


const stack = pulumi.getStack();

// build and pull images
const backendImageName = "backend";
const backend = new docker.Image("backend", {
  build: {
    context: `${process.cwd()}/app/backend`,
  },
  imageName: `${backendImageName}:${stack}`,
  skipPush: true,
});

const frontendImageName = "frontend";
const frontend = new docker.Image("frontend", {
  build: {
    context: `${process.cwd()}/app/frontend`,
  },
  imageName: `${frontendImageName}:${stack}`,
  skipPush: true,
});

const mongoImage = new docker.RemoteImage("mongo", {
  name: "mongo:bionic",
  keepLocally: true,
})


// create a network
const network = new docker.Network("network", {
  name: `services-${stack}`,
});

// create container instances
const mongoContainer = new docker.Container("mongoContainer", {
  image: mongoImage.repoDigest,
  name: `mongo-${stack}`,
  networksAdvanced: [
    {
      name: network.name,
      aliases: ["mongo"],
    },
  ],
  envs: [
    `MONGO_INITDB_ROOT_USERNAME=${mongoUsername}`,
    pulumi.interpolate`MONGO_INITDB_ROOT_PASSWORD=${mongoPassword}`,
  ]
});

const backendContainer = new docker.Container("backendContainer", {
  name: `backend-${stack}`,
  image: backend.baseImageName,
  envs: [
    pulumi.interpolate`DATABASE_HOST=mongodb://${mongoUsername}:${mongoPassword}@${mongoHost}:${mongoPort}`,
    `DATABASE_NAME=${database}?authSource=admin`,
    `NODE_ENV=${nodeEnvironment}`,
  ],
  networksAdvanced: [
    {
      name: network.name,
    },
  ],
}, { dependsOn: [mongoContainer] });

const frontendContainer = new docker.Container("frontendContainer", {
  image: frontend.baseImageName,
  name: `frontend-${stack}`,
  ports: [
    {
      internal: 3001,
      external: frontendPort,
    },
  ],
  envs: [
    `LISTEN_PORT=${frontendPort}`,
    `HTTP_PROXY=backend-${stack}:${backendPort}`,
  ],
  networksAdvanced: [
    {
      name: network.name,
    },
  ],
});

const dataSeedContainer = new docker.Container("dataSeedContainer", {
  image: mongoImage.repoDigest,
  name: `dataSeed-${stack}`,
  mustRun: false,
  //rm: true,  // if uncommented resource needs to be manually removed from state
  mounts: [
    {
      target: "/home/products.json",
      type: "bind",
      source: `${process.cwd()}/app/data/products.json`,
    },
  ],
  command: [
    "sh",
    "-c",
    "mongoimport --host ${mongoHost} -u ${mongoUsername} -p ${mongoPassword} --authentication admin --db cart --collection products --type json --file /home/products.json --jsonArray",
  ],
  networksAdvanced: [
    {
      name: network.name,
    },
  ],
});



export const url = pulumi.interpolate`http://localhost:${frontendPort}`