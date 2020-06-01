module github.com/netdata/sd

go 1.14

require (
	github.com/fsnotify/fsnotify v1.4.7
	github.com/imdario/mergo v0.3.9 // indirect
	github.com/mattn/go-isatty v0.0.12
	github.com/mitchellh/hashstructure v1.0.0
	github.com/stretchr/testify v1.5.1
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/utils v0.0.0-20200414100711-2df71ebbae66 // indirect
)

replace k8s.io/client-go => k8s.io/client-go v0.18.1
