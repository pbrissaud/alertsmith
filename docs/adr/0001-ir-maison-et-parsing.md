# IR maison + parsing en un passage via structs typés à feuilles `yaml.Node`

Les deux formats d'entrée (flat natif + CRD `monitoring.coreos.com/v1`) sont normalisés vers une **IR maison** qui porte la provenance (fichier + ligne) **à la feuille**, pas à la racine — sinon un fichier YAML multi-document casse le modèle. Le parsing se fait en **un seul passage** dans des structs typés dont les champs-feuilles sont des `yaml.Node` (le pattern de `rulefmt`), ce qui donne la position **au niveau du champ** sans walker générique. `rulefmt` et `promql/parser` ne sont **pas** sur le chemin d'unmarshaling : ils servent uniquement de **validateurs** à qui l'on passe les valeurs reconstruites. Le moteur en aval tourne exclusivement sur l'IR maison.

## Considered options

- **Utiliser `rulefmt.RuleGroups` / `monitoringv1.PrometheusRule` comme IR** — rejeté : on perd les positions ligne/colonne (nécessaires aux **Comments** inline) et l'enveloppe K8s ; il faudrait re-parser le fichier une 2e fois pour les retrouver.
- **Walk générique d'un arbre `yaml.Node`** — reporté post-V1 : seulement utile pour les positions **sub-champ** (pointer un token dans une expr PromQL), plus lourd que le pattern structs-typés-à-feuilles-node.
- **Dépendre de `monitoringv1` (prometheus-operator)** — rejeté, raison principale : **inutile**. Lire `expr` comme `yaml.Node` absorbe déjà le cas `intstr.IntOrString` (`expr: 0` en int vs string), donc la dépendance n'apporte rien. (Le poids de `k8s.io/api` est un bonus, *pas* l'argument porteur : l'ADR 0009 assume tout `prometheus/prometheus`, arbre notoirement plus lourd — « lourd » ne tient donc pas seul comme motif.)

## Consequences

- V1 : positions **au niveau du champ**. Un pointage sub-champ (token dans le PromQL) exigera le walk générique reporté.
- Détection de format **par contenu** (`apiVersion` + `kind`), pas par path. Puis : flat walke `groups:` racine, CRD walke `spec.groups`, avec la même struct interne de rule-group en aval.
