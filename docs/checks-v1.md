# Checks déterministes V1 — la « jurisprudence » de base

> **Statut** : spec/backlog vivant, *pas* un ADR. Chaque check est réversible (ajout/retrait trivial). En revanche, **deux contrats sont gelés** ici : les **IDs de check** (clés de `.alertsmith.yaml` 0004 + cible des Suppressions 0005) et la **grammaire de Suppression**.
>
> Rappels de cadre : tous ces checks sont des **mécanismes** → OSS/gratuit pour toujours (0012/0013). Aucun ne consomme le **Ruleset** payant en V1 — le premier consommateur sera l'heuristique catalogue (étape 4) puis le LLM. `Level` par défaut **possédé par le check en code** (0004) ; `Blocking = enforceable ET Level ≥ block_on` (0006).

## Registre des checks V1

| ID | Ce qu'il vérifie | Level | Enf. | Défaut |
|---|---|---|---|---|
| `promql-parse` | l'`expr` (alerting/recording) parse via `promql/parser` | `error` | enforceable | on |
| `rule-structure` | structure rulefmt valide : champs requis, nom de groupe non vide, groupe non vide | `error` | enforceable | on |
| `yaml-malformed` | le fichier ne parse pas **sans** `{{` (YAML réellement cassé — 0002) | `error` | enforceable | on |
| `helm-skipped` | fichier templaté Helm sauté (0002) | `note` | **advisory** | on |
| `duplicate-rule` | même `alert:` (ou `record:`) dupliqué dans un groupe | `warning` | enforceable | on |
| `missing-for` | alerting rule sans `for:` (fire sur un seul scrape → flapping-prone) | `warning` | enforceable | on |
| `missing-severity` | alerting rule sans label `severity` | `warning` | enforceable | on |
| `invalid-severity` | label `severity` hors du set configuré (`warning`/`critical`/…) | `warning` | enforceable | on |
| `missing-summary` | annotation `summary` absente | `warning` | enforceable | on |
| `missing-description` | annotation `description` absente | `note` | enforceable | on |
| `missing-runbook` | annotation `runbook_url` absente | `note` | enforceable | **off** |
| `annotation-template-parse` | annotations = Go `text/template` valides (`{{ $labels.x }}`, `{{ $value }}`) | `warning` | enforceable | on |
| `missing-recording-rule` | l'expr réfère un nom **avec `:`** qu'aucun `record:` du repo ne définit (0003) | `warning` | enforceable¹ | on |
| `recording-rule-naming` | un `record:` ne suit pas la convention `niveau:métrique:opération` | `note` | **advisory** | off |

¹ `missing-recording-rule` est **downgradé en advisory** quand l'index `record:` est connu-incomplet (fichiers skippés/malformés — 0003), pour ne pas fabriquer de faux positif bloquant.

### Notes

- **`missing-runbook` off par défaut** : beaucoup d'équipes n'ont pas de runbook public par alerte ; l'activer est un choix org (0004).
- **`annotation-template-parse`** : les annotations Prometheus sont du Go templating — une accolade non fermée ou un `.` invalide casse le rendu *à la notification*, invisible sinon. `promtool` le vérifie déjà côté lib.
- **`invalid-severity`** consomme le set autorisé depuis `.alertsmith.yaml` (donnée de *config*, pas le Ruleset payant).
- Les checks `error`/enforceable = la « base gratuite de promtool » (0009), obtenue en lib.

## Grammaire de Suppression (contrat public gelé — 0005)

```
# alertsmith:ignore <check-id>[,<check-id>…] [-- <raison>]
```

- **Attachement** : commentaire en **HeadComment** (ligne(s) au-dessus) *ou* **LineComment** (fin de ligne) de la règle visée — lu via les commentaires portés par `yaml.Node`, cohérent avec le parse unique (0001).
- **Portée** : la **règle attachée uniquement** (pas de portée groupe/fichier en V1).
- `<check-id>` : un ou plusieurs, séparés par **virgule sans espace** (`missing-for,missing-severity`).
- `-- <raison>` : optionnelle mais **recommandée** (versionnée + reviewable en PR — c'est tout l'intérêt du stateless, 0005).
- **Case-sensitive**, kebab-case, doit matcher un ID du registre.

Exemple :
```yaml
- alert: HighLatency
  # alertsmith:ignore missing-runbook,missing-description -- SLO interne, pas de runbook public
  expr: rate(http_request_duration_seconds_sum[5m]) > 0.5
  for: 10m
```

## Méta-checks (le reviewer se surveille)

| ID | Ce qu'il vérifie | Level | Enf. |
|---|---|---|---|
| `unknown-suppression` | une Suppression réfère un `check-id` inconnu (typo → suppression morte silencieuse) | `note` | advisory |
| `stale-suppression` *(optionnel V1.5)* | une Suppression ne matche **aucun** Finding émis (règle corrigée, suppression oubliée) | `note` | advisory |

`stale-suppression` est marqué optionnel : utile pour l'hygiène, mais potentiellement bruyant → à activer une fois le reste stabilisé.

## Explicitement HORS V1 (pointeurs)

- **LLM-only, advisory** (0006) : flapping, seuil sensé/arbitraire, actionnabilité, cohérence severity. Nourri du contexte déterministe.
- **Catalogue statique** (étape 4, *premier consommateur du Ruleset*) : métrique **brute sans `:`** qui ne matche aucun exporter connu → typo probable.
- **Grounding** (0008, payant) : métrique absente du TSDB live du client, corrélé sur le runner.
- **Coverage** (« ce Deployment n'a aucune alerte ») : advisory par nature, classe grounding/contexte, V2.
- **`ruleSelector` mismatch** (doc, V2) : règles jamais chargées faute de matcher le `Prometheus` operator — contexte externe (autre repo).
- **dead-recording-rule** (0003) : « non référencé intra-repo » ≠ « mort » → classe grounding.
