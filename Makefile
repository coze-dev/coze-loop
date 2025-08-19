IMAGE_REGISTRY := docker.io
IMAGE_REPOSITORY := cozedev
IMAGE_NAME := coze-loop

DOCKER_COMPOSE_DIR := ./release/deployment/docker-compose

HELM_CHART_DIR := ./release/deployment/helm-chart/umbrella
HELM_NAMESPACE := coze-loop
HELM_RELEASE := coze-loop

.PHONY: image mini-start mini-tunnel

.PHONY: FORCE
FORCE:

image%:
	@case "$*" in \
	  -login) \
	    docker login $(IMAGE_REGISTRY) -u $(IMAGE_REPOSITORY) ;; \
	  -bpush-*) \
	    version="$*"; \
        version="$${version#-bpush-}"; \
	    docker buildx build \
		  --platform linux/amd64,linux/arm64 \
		  --progress=plain \
		  --push \
		  -f ./release/image/Dockerfile \
		  -t $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):latest \
		  -t $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):"$$version" \
		  .; \
		docker pull $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):latest; \
		docker run --rm $(IMAGE_REPOSITORY)/$(IMAGE_NAME):latest du -sh /coze-loop/bin; \
		docker run --rm $(IMAGE_REPOSITORY)/$(IMAGE_NAME):latest du -sh /coze-loop/resources; \
		docker run --rm $(IMAGE_REPOSITORY)/$(IMAGE_NAME):latest du -sh /coze-loop ;; \
	  -help|*) \
      	echo "Usage:"; \
      	echo "  make image--login             # Login to the image registry ($(IMAGE_REGISTRY))"; \
      	echo "  make image-<version>          # Build & push multi-arch image with tags <version> and latest"; \
      	echo; \
      	echo "Examples:"; \
      	echo "  make image--login             # Login before pushing images"; \
      	echo "  make image-1.0.0              # Build & push images tagged '1.0.0' and 'latest'"; \
      	echo; \
      	echo "Notes:"; \
      	echo "  - 'image--login' logs in using IMAGE_REPOSITORY as the username."; \
      	echo "  - 'image-<version>' will push to $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME)"; \
      	exit 1 ;; \
	esac

compose%:
	@case "$*" in \
	  -up) \
	    docker compose \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose.yml \
	      --env-file $(DOCKER_COMPOSE_DIR)/.env \
	      --profile "*" \
	      up ;; \
	  -restart-*) \
	    svc="$*"; \
	    svc="$${svc#-restart-}"; \
	    docker compose \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose.yml \
	      --env-file $(DOCKER_COMPOSE_DIR)/.env \
	      restart "$$svc" ;; \
	  -down) \
	    docker compose \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose.yml \
	      --env-file $(DOCKER_COMPOSE_DIR)/.env \
	      --profile "*" \
	      down ;; \
	  -down-v) \
	    docker compose \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose.yml \
	      --env-file $(DOCKER_COMPOSE_DIR)/.env \
	      --profile "*" \
	      down -v ;; \
	  -up-dev) \
	    docker compose \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose.yml \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose-dev.yml \
	      --env-file $(DOCKER_COMPOSE_DIR)/.env \
	      --profile "*" \
	      up --build  ;; \
	  -restart-dev-*) \
		svc="$*"; \
		svc="$${svc#-restart-dev-}"; \
		docker compose \
		  -f $(DOCKER_COMPOSE_DIR)/docker-compose.yml \
          -f $(DOCKER_COMPOSE_DIR)/docker-compose-dev.yml \
		  --env-file $(DOCKER_COMPOSE_DIR)/.env \
		  restart "$$svc" ;; \
	  -down-dev) \
	    docker compose \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose.yml \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose-dev.yml \
	      --env-file $(DOCKER_COMPOSE_DIR)/.env \
	      --profile "*" \
	      down ;; \
	  -down-v-dev) \
	    docker compose \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose.yml \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose-dev.yml \
	      --env-file $(DOCKER_COMPOSE_DIR)/.env \
	      --profile "*" \
	      down -v ;; \
	  -up-debug) \
	    docker compose \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose.yml \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose-debug.yml \
	      --env-file $(DOCKER_COMPOSE_DIR)/.env \
	      --profile "*" \
	      up --build  ;; \
	  -restart-debug-*) \
		svc="$*"; \
		svc="$${svc#-restart-debug-}"; \
		docker compose \
		  -f $(DOCKER_COMPOSE_DIR)/docker-compose.yml \
			-f $(DOCKER_COMPOSE_DIR)/docker-compose-debug.yml \
		  --env-file $(DOCKER_COMPOSE_DIR)/.env \
		  restart "$$svc" ;; \
	  -down-debug) \
	    docker compose \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose.yml \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose-debug.yml \
	      --env-file $(DOCKER_COMPOSE_DIR)/.env \
	      --profile "*" \
	      down ;; \
	  -down-v-debug) \
	    docker compose \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose.yml \
	      -f $(DOCKER_COMPOSE_DIR)/docker-compose-debug.yml \
	      --env-file $(DOCKER_COMPOSE_DIR)/.env \
	      --profile "*" \
	      down -v ;; \
	  -help|*) \
      	echo "Usage:"; \
      	echo "  # Stable profile"; \
      	echo "  make compose-up                   # Start base services"; \
      	echo "  make compose-restart-<svc>        # Restart specific base service"; \
      	echo "  make compose-down                 # Stop base services"; \
      	echo "  make compose-down-v               # Stop base services and remove volumes"; \
      	echo; \
      	echo "  # Dev profile"; \
      	echo "  make compose-up-dev               # Start base + dev services (build)"; \
      	echo "  make compose-restart-dev-<svc>    # Restart specific dev service"; \
      	echo "  make compose-down-dev             # Stop base + dev services"; \
      	echo "  make compose-down-v-dev           # Stop base + dev services and remove volumes"; \
      	echo; \
      	echo "  # Debug profile"; \
      	echo "  make compose-up-debug             # Start base + debug services (build)"; \
      	echo "  make compose-restart-debug-<svc>  # Restart specific debug service"; \
      	echo "  make compose-down-debug           # Stop base + debug services"; \
      	echo "  make compose-down-v-debug         # Stop base + debug services and remove volumes"; \
      	echo; \
      	echo "Notes:"; \
      	echo "  - '<svc>' means the name of a service in docker-compose.yml"; \
      	echo "  - '--profile \"*\"' is only needed for 'up', not for 'down' or 'restart'."; \
      	echo "  - If you used multiple -f files for 'up', use the same -f set for 'down' or 'restart'."; \
      	exit 1 ;; \
	esac

helm%:
	@case "$*" in \
	  -login) \
      	helm registry login $(IMAGE_REGISTRY) -u $(IMAGE_REPOSITORY) ;; \
	  -chart-deps) \
	    helm dependency build $(HELM_CHART_DIR) ;; \
	  -chart-deps-clean) \
		rm -rf $(HELM_CHART_DIR)/charts $(HELM_CHART_DIR)/Chart.lock ;; \
	  -chart-bpush-*) \
	    version="$*"; \
        version="$${version#-chart-bpush-}"; \
        helm dependency build $(HELM_CHART_DIR); \
        helm package $(HELM_CHART_DIR) --version "$$version"-helm; \
       	helm push $(IMAGE_NAME)-"$$version"-helm.tgz oci://$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY); \
       	rm -f $(IMAGE_NAME)-"$$version"-helm.tgz; \
        rm -rf $(HELM_CHART_DIR)/charts $(HELM_CHART_DIR)/Chart.lock ;; \
	  -ctx) \
	    kubectl config get-contexts ;; \
	  -ctx-*) \
		ctx="$*"; \
		ctx="$${ctx#-ctx-}"; \
		echo "switch to context: $$ctx"; \
		kubectl config use-context "$$ctx" ;; \
	  -ns) \
	    kubectl get namespaces ;; \
	  -pod) \
	    kubectl get pods -n $(HELM_NAMESPACE) ;; \
	  -svc) \
	    kubectl get svc -n $(HELM_NAMESPACE) -o wide ;; \
	  -ingress) \
	    kubectl get ingress -n $(HELM_NAMESPACE) ;; \
	  -up) \
		helm upgrade \
		  --install --force $(HELM_RELEASE) $(HELM_CHART_DIR) \
		  --namespace $(HELM_NAMESPACE) --create-namespace \
		  -f $(HELM_CHART_DIR)/values.yaml ;; \
	  -up-exp-minikube-*) \
	    vals="$*"; \
		vals="$${vals#-up-exp-minikube-}"; \
		helm upgrade \
		  --install --force $(HELM_RELEASE) $(HELM_CHART_DIR) \
		  --namespace $(HELM_NAMESPACE) --create-namespace \
		  -f $(HELM_CHART_DIR)/examples/minikube/"$$vals".values.yaml ;; \
	  -down) \
	    helm list -n $(HELM_NAMESPACE) -q \
	    | \
	    xargs -r -n1 helm uninstall -n $(HELM_NAMESPACE) ;; \
	  -logf-*) \
      	app="$*"; \
      	app="$${app#-logf-}"; \
      	kubectl -n $(HELM_NAMESPACE) logs \
      	  -l app=$(HELM_RELEASE)-$$app \
      	  --all-containers=true \
      	  --tail=100 \
      	  --prefix=true \
		  --max-log-requests=10 \
      	  -f ;; \
	  -tpl-*) \
      	app="$*"; \
      	app="$${app#-tpl-}"; \
      	helm template $(HELM_RELEASE) $(HELM_CHART_DIR) \
      	  --namespace $(HELM_NAMESPACE) \
      	  -f $(HELM_CHART_DIR)/values.yaml | \
      	APP="$$app" yq eval '. | select(.kind == "Deployment" and .metadata.name == ("coze-loop-" + strenv(APP)))' - ;; \
	  -help|*) \
	  	echo "Usage:"; \
	  	echo; \
	  	echo "  # Auth & Chart packaging"; \
	  	echo "  make helm-login                    # OCI login to registry ($(IMAGE_REGISTRY)) using user=$(IMAGE_REPOSITORY)"; \
	  	echo "  make helm-chart-deps               # Build chart dependencies (helm dependency build)"; \
	  	echo "  make helm-chart-deps-clean         # Clean deps: remove charts/ and Chart.lock"; \
	  	echo "  make helm-chart-bpush-<version>    # Package chart as <version>-helm and push to OCI ($(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY))"; \
	  	echo; \
	  	echo "  # Kube context & namespace"; \
	  	echo "  make helm-ctx                      # List kube contexts"; \
	  	echo "  make helm-ctx-<context>            # Switch to kube context <context>"; \
	  	echo "  make helm-ns                       # List namespaces"; \
	  	echo; \
	  	echo "  # Inspect resources in namespace $(HELM_NAMESPACE)"; \
	  	echo "  make helm-pod                      # List pods (wide)"; \
	  	echo "  make helm-svc                      # List services (wide)"; \
	  	echo "  make helm-ingress                  # List ingress resources"; \
	  	echo; \
	  	echo "  # Release lifecycle"; \
	  	echo "  make helm-up                       # helm upgrade --install $(HELM_RELEASE) from $(HELM_CHART_DIR) (uses values.yaml)"; \
	  	echo "  make helm-up-exp-minikube-<vals>   # helm upgrade using examples/minikube/<vals>.values.yaml"; \
	  	echo "  make helm-down                     # Uninstall ALL releases in namespace $(HELM_NAMESPACE)"; \
	  	echo; \
	  	echo "  # Logs & templating"; \
	  	echo "  make helm-logf-<app>               # Follow logs for pods with label app=$(HELM_RELEASE)-<app>, all containers"; \
	  	echo "  make helm-tpl-<app>                # Render only Deployment coze-loop-<app> to stdout (no apply)"; \
	  	echo; \
	  	echo "Examples:"; \
	  	echo "  make helm-login"; \
	  	echo "  make helm-chart-deps && make helm-chart-bpush-1.0.0"; \
	  	echo "  make helm-ctx && make helm-ctx-minikube"; \
	  	echo "  make helm-up     # installs/updates $(HELM_RELEASE) in $(HELM_NAMESPACE)"; \
	  	echo "  make helm-logf-app   # e.g., app=api => label app=$(HELM_RELEASE)-api"; \
	  	echo; \
	  	echo "Notes:"; \
	  	echo "  - Ensure HELM_NAMESPACE and HELM_RELEASE are exported or set in the environment."; \
	  	echo "  - helm-chart-bpush-<version> produces <chart>-<version>-helm.tgz then pushes and cleans local artifact."; \
	  	echo "  - Template filter expects Deployment.metadata.name = \"coze-loop-<app>\"."; \
	  	exit 1 ;; \
	esac

minikube%:
	@case "$*" in \
	  -start) \
		minikube start --addons=ingress ;; \
	  -tunnel) \
		sudo minikube tunnel ;; \
	  -help|*) \
	  	echo "Usage:"; \
	  	echo; \
	  	echo "  make minikube-start       # Start minikube with ingress addon enabled"; \
	  	echo "  make minikube-tunnel      # Run minikube tunnel (requires sudo), exposes LoadBalancer/Ingress services locally"; \
	  	echo; \
	  	echo "Examples:"; \
	  	echo "  make minikube-start"; \
	  	echo "  make minikube-tunnel"; \
	  	echo; \
	  	echo "Notes:"; \
	  	echo "  - 'minikube-start' uses '--addons=ingress' to enable NGINX ingress controller automatically."; \
	  	echo "  - 'minikube-tunnel' will bind service external IPs to localhost for LoadBalancer/Ingress access."; \
	  	echo "  - 'minikube-tunnel' may require admin privileges (sudo) depending on your OS/network setup."; \
	  	exit 1 ;; \
	esac