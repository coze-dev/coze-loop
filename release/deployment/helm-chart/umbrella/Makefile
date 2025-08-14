HELM_CHART_DIR := .
HELM_NAMESPACE := coze-loop
HELM_RELEASE := coze-loop

.PHONY: FORCE
FORCE:

helm%:
	@case "$*" in \
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
	  	echo "  # Kube context & namespaces"; \
	  	echo "  make helm-ctx                    # List kube contexts"; \
	  	echo "  make helm-ctx-<context>          # Switch to kube context <context>"; \
	  	echo "  make helm-ns                     # List namespaces"; \
	  	echo; \
	  	echo "  # Inspect resources in namespace $(HELM_NAMESPACE)"; \
	  	echo "  make helm-pod                    # List pods"; \
	  	echo "  make helm-svc                    # List services (wide)"; \
	  	echo "  make helm-ingress                # List ingress resources"; \
	  	echo; \
	  	echo "  # Release lifecycle (chart dir: $(HELM_CHART_DIR), release: $(HELM_RELEASE))"; \
	  	echo "  make helm-up                     # helm upgrade --install from $(HELM_CHART_DIR) using values.yaml"; \
		echo "  make helm-up-exp-minikube-<vals> # helm upgrade using examples/minikube/<vals>.values.yaml"; \
	  	echo "  make helm-down                   # Uninstall ALL releases in namespace $(HELM_NAMESPACE)"; \
	  	echo; \
	  	echo "  # Logs & templating"; \
	  	echo "  make helm-logf-<app>             # Follow logs of pods with label app=$(HELM_RELEASE)-<app>, all containers"; \
	  	echo "  make helm-tpl-<app>              # Render Deployment named coze-loop-<app> (no apply)"; \
	  	echo; \
	  	echo "Examples:"; \
	  	echo "  make helm-ctx && make helm-ctx-minikube"; \
	  	echo "  make helm-up      # install/update $(HELM_RELEASE) in $(HELM_NAMESPACE)"; \
	  	echo "  make helm-logf-api   # follows pods labeled app=$(HELM_RELEASE)-api"; \
	  	echo "  make helm-tpl-api    # prints Deployment coze-loop-api YAML"; \
	  	echo; \
	  	echo "Notes:"; \
	  	echo "  - Ensure HELM_NAMESPACE and HELM_RELEASE are set before running commands."; \
	  	echo "  - Commands with '-<name>' suffix accept a dynamic argument (e.g., helm-ctx-xxx, helm-logf-xxx)."; \
	  	echo "  - '-tpl-*' renders manifests locally without applying to the cluster."; \
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