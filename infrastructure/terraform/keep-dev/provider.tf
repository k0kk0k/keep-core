data "google_client_config" "default" {}

# Configure the Google Cloud provider
provider "google" {
  version = "<= 1.19.0"
  region  = "${var.region_data["region"]}"
}

provider "google-beta" {
  version = "<= 1.19.0"
  region  = "${var.region_data["region"]}"
}

provider "kubernetes" {
  version                = "<= 1.5.0"
  load_config_file       = false
  host                   = "https://${var.gke_cluster["master_private_endpoint"]}"
  token                  = "${data.google_client_config.default.access_token}"
  cluster_ca_certificate = "${base64decode(module.gke_cluster.cluster_ca_certificate)}"
}

module "helm_provider_helper" {
  source                = "git@github.com:thesis/infrastructure.git//terraform/modules/helm_tiller_helper"
  tiller_namespace_name = "${var.tiller_namespace_name}"
}

provider "helm" {
  version = "<= 0.10.2"

  kubernetes {
    host                   = "https://${var.gke_cluster["master_private_endpoint"]}"
    token                  = "${data.google_client_config.default.access_token}"
    cluster_ca_certificate = "${base64decode(module.gke_cluster.cluster_ca_certificate)}"
  }

  tiller_image    = "gcr.io/kubernetes-helm/tiller:v2.14.2"
  service_account = "${module.helm_provider_helper.tiller_service_account}"
  override        = ["spec.template.spec.automountserviceaccounttoken=true"]
  namespace       = "${module.helm_provider_helper.tiller_namespace}"
  install_tiller  = true
}

provider "null" {
  version = "<= 2.0.0"
}

provider "random" {
  version = "<= 2.0.0"
}

provider "template" {
  version = "<= 1.0.0"
}
