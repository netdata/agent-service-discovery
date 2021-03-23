module github.com/netdata/sd

go 1.16

replace k8s.io/client-go => k8s.io/client-go v0.18.3

require (
	github.com/Masterminds/sprig/v3 v3.1.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/gobwas/glob v0.2.3
	github.com/google/uuid v1.1.2 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/ilyam8/hashstructure v1.1.0
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jessevdk/go-flags v1.4.0
	github.com/mattn/go-isatty v0.0.12
	github.com/rs/zerolog v1.18.0
	github.com/stretchr/testify v1.6.0
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/utils v0.0.0-20200529193333-24a76e807f40 // indirect
)
