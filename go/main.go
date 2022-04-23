package main

import (
	"os"

	"github.com/pulumi/pulumi-docker/sdk/v3/go/docker"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		config := config.New(ctx, "")

		frontendPort := config.RequireInt("frontend_port")
		backendPort := config.RequireInt("backend_port")
		mongoPort := config.RequireInt("mongo_port")
		mongoHost := config.Require("mongo_host")
		database := config.Require("database")
		nodeEnvironment := config.Require("node_environment")
		mongoUsername := config.Require("mongo_username")
		mongoPassword := config.RequireSecret("mongo_password")

		stack := ctx.Stack()

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		// build and pull images
		const backendImageName = "backend-go"
		backendImage, err := docker.NewImage(ctx, "backend", &docker.ImageArgs{
			ImageName: pulumi.Sprintf("%s:%s", backendImageName, stack),
			Build: &docker.DockerBuildArgs{
				Context: pulumi.Sprintf("%s/app/backend", cwd),
			},
			Registry: &docker.ImageRegistryArgs{},
			SkipPush: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}

		const frontendImageName = "frontend-go"
		frontendImage, err := docker.NewImage(ctx, "frontend", &docker.ImageArgs{
			ImageName: pulumi.Sprintf("%s:%s", frontendImageName, stack),
			Build: &docker.DockerBuildArgs{
				Context: pulumi.Sprintf("%s/app/frontend", cwd),
			},
			Registry: &docker.ImageRegistryArgs{},
			SkipPush: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}

		mongoImage, err := docker.NewRemoteImage(ctx, "mongo", &docker.RemoteImageArgs{
			Name:        pulumi.String("mongo:bionic"),
			KeepLocally: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}

		// create a network
		network, err := docker.NewNetwork(ctx, "network", &docker.NetworkArgs{
			Name: pulumi.Sprintf("services-%s", stack),
		})
		if err != nil {
			return err
		}

		// create container instances
		mongoContainer, err := docker.NewContainer(ctx, "mongoContainer", &docker.ContainerArgs{
			Envs: &pulumi.StringArray{
				pulumi.Sprintf("MONGO_INITDB_ROOT_USERNAME=%s", mongoUsername),
				pulumi.Sprintf("MONGO_INITDB_ROOT_PASSWORD=%s", mongoPassword),
			},
			Image: mongoImage.RepoDigest,
			Name:  pulumi.Sprintf("mongo-%s", stack),
			NetworksAdvanced: &docker.ContainerNetworksAdvancedArray{
				&docker.ContainerNetworksAdvancedArgs{
					Aliases: &pulumi.StringArray{pulumi.String("mongo")},
					Name:    network.Name,
				},
			},
		})
		if err != nil {
			return err
		}

		_, err = docker.NewContainer(ctx, "backendContainer", &docker.ContainerArgs{
			Envs: &pulumi.StringArray{
				pulumi.Sprintf("DATABASE_HOST=mongodb://%s:%s@%s:%v", mongoUsername, mongoPassword, mongoHost, mongoPort),
				pulumi.Sprintf("DATABASE_NAME=%s?authSource=admin", database),
				pulumi.Sprintf("NODE_ENV=%s", nodeEnvironment),
			},
			Image: backendImage.BaseImageName,
			Name:  pulumi.Sprintf("backend-%s", stack),
			NetworksAdvanced: &docker.ContainerNetworksAdvancedArray{
				&docker.ContainerNetworksAdvancedArgs{
					Name: network.Name,
				},
			},
		}, pulumi.DependsOn([]pulumi.Resource{mongoContainer}))
		if err != nil {
			return err
		}

		_, err = docker.NewContainer(ctx, "frontendContainer", &docker.ContainerArgs{
			Envs: &pulumi.StringArray{
				pulumi.Sprintf("LISTEN_PORT=%s", frontendPort),
				pulumi.Sprintf("HTTP_PROXY=backend-%s:%v", stack, backendPort),
			},
			Image: frontendImage.BaseImageName,
			Name:  pulumi.Sprintf("frontend-%s", stack),
			NetworksAdvanced: &docker.ContainerNetworksAdvancedArray{
				&docker.ContainerNetworksAdvancedArgs{
					Name: network.Name,
				},
			},
			Ports: &docker.ContainerPortArray{
				&docker.ContainerPortArgs{
					External: pulumi.Int(frontendPort),
					Internal: pulumi.Int(3001),
					Protocol: pulumi.String("TCP"),
				},
			},
		})
		if err != nil {
			return err
		}

		_, err = docker.NewContainer(ctx, "dataSeedContainer", &docker.ContainerArgs{
			Command: pulumi.StringArray{
				pulumi.String("sh"),
				pulumi.String("-c"),
				pulumi.Sprintf("mongoimport --host %s -u %s -p %s --authentication admin --db cart --collection products --type json --file /home/products.json --jsonArray", mongoHost, mongoUsername, mongoPassword),
			},
			Image: mongoImage.RepoDigest,
			Mounts: docker.ContainerMountArray{
				&docker.ContainerMountArgs{
					Source: pulumi.Sprintf("%s/app/data/products.json", cwd),
					Target: pulumi.String("/home/products.json"),
					Type:   pulumi.String("bind"),
				},
			},
			MustRun: pulumi.Bool(false),
			Name:    pulumi.Sprintf("dataSeed-%s", stack),
			NetworksAdvanced: &docker.ContainerNetworksAdvancedArray{
				&docker.ContainerNetworksAdvancedArgs{
					Name: network.Name,
				},
			},
			//Rm: pulumi.Bool(true),
		}, pulumi.DependsOn([]pulumi.Resource{mongoContainer}))
		if err != nil {
			return err
		}

		// outputs
		ctx.Export("url", pulumi.Sprintf("http://localhost:%v", frontendPort))

		return nil
	})
}
