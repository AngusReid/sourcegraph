# Deployment

This document describes how Sourcegraph is deployed.

## Types of deployment

There are two ways customers deploy Sourcegraph: Server and Data Center.

### Server ([code](https://sourcegraph.sgdev.org/github.com/sourcegraph/sourcegraph/-/tree/cmd/server) | [docs](https://about.sourcegraph.com/docs))

The Server deployment is a Docker image that can be run on a single node with a simple command that is documented on our home page. It is the free and easy way for a customer to start using Sourcegraph.

### Data Center ([code and docs](https://sourcegraph.sgdev.org/github.com/sourcegraph/deploy-sourcegraph))

The Data Center deployment is a paid upgrade and allows customers to deploy Sourcegraph across a cluster of machines using Kubernetes.

## Locations

Sourcegraph is deployed in multiple locations.

### Customers

When customers want Sourcegraph to work on their private code, they deploy Sourcegraph Server or Data Center on their own infrastructure using our public documentation.

### Production (https://sourcegraph.com)

Production is a public demonstration of Sourcegraph for all public code on GitHub.

We take shortcuts to make it work at that scale (tens of millions of repos). Our primary focus is making Sourcegraph work at customer scale (tens of thousands of repos).

The deployment is similar to Data Center in that it uses Kubernetes, but it pre-dates our deploy-sourcegraph process so there are some quirks ([code](https://sourcegraph.sgdev.org/github.com/sourcegraph/infrastructure/-/tree/kubernetes/cmd/sg-gen-prod)).

### Dogfood (https://sourcegraph.sgdev.org)

Dogfood is a private Data Center deployment for all of our private code, just like what customers set up for themselves.

## Release process

### Dogfood and production

Commits to the master branch of github.com/sourcegraph/sourcegraph are continuously deployed to our frontend service in production and in dogfood.

Other core services are automatically deployed when commits are pushed to a branch with the prefix `docker-images/`.

* e.g. to deploy gitserver
  ```
  git checkout master
  git pull
  git push origin master:docker-images/gitserver
  ```

### To our customers

We ship to our customers minor feature releases monthly (e.g. 2.7, 2.8, 2.9), and patch releases on an as-needed basis (e.g. 2.8.1).

* [Server release process](https://sourcegraph.sgdev.org/github.com/sourcegraph/sourcegraph/-/blob/cmd/server/README.md)
* [Data Center release process](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph/-/blob/README.dev.md)