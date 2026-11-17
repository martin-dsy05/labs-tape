# Practica CI/CD de Tape

## Validacion automatica

El flujo ejecuta `go vet`, pruebas locales y `go test ./...` antes de construir
la imagen. Las pruebas de integracion usan un registro TLS local de Trex:
resuelven referencias por etiqueta y digest, buscan etiquetas relacionadas,
copian imagenes, actualizan manifiestos y empaquetan artefactos. Ninguna de esas
pruebas se marca como aprobada por omitir sus aserciones.

Las imagenes se generan en memoria para las pruebas. Los contenidos asociados a
las etiquetas `.sig`, `.att` y `.sbom` son sinteticos: sirven para comprobar su
descubrimiento, no validan firmas ni documentos SBOM reales. Los manifiestos
temporales tienen su propio repositorio Git para comprobar procedencia sin
modificar el directorio de trabajo del usuario.

## Compatibilidad con registros publicos

Los casos originales de Contour, Flux y Tekton se conservan. Para ejecutarlos:

```sh
TAPE_EXTERNAL_TESTS=1 go test -mod=vendor -count=1 -timeout=10m ./...
```

Tambien pueden seleccionarse desde Run workflow, activando
`external_registries`. Requieren acceso a sus imagenes publicas. En la primera
ejecucion el registro `gcr.io` rechazo una imagen de Tekton con `DENIED`.
La aprobacion de la suite local no demuestra compatibilidad actual con esas
imagenes ni corrige sus permisos de acceso.

## Publicacion

El trabajo de imagen depende de las comprobaciones. En ramas y pull requests
construye y comprueba `tape --help` sin publicar. Solo un push a `main` con
comprobaciones satisfactorias puede publicar en `ghcr.io/<cuenta>/<repositorio>`.
La etiqueta incluye el SHA del commit. La comprobacion de ayuda verifica el
arranque del CLI; no demuestra un empaquetado ni un despliegue en Kubernetes.
