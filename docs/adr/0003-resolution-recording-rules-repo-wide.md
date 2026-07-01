# Résolution des refs de recording rules : index repo-wide, review sur le diff

Le check « alerte → recording rule inexistante / mal orthographiée » a besoin de savoir quelles recording rules **existent** dans tout le repo, car une recording rule peut être définie dans un fichier que la PR ne touche pas. Donc **deux portées distinctes** : la **résolution** indexe *tous* les `record:` du repo (tous les fichiers `PrometheusRule` sur disque, gratuits via `actions/checkout`), alors que la **review** n'émet des Findings que sur les règles du **diff** de la PR.

## Scope du check (V1)

- **Convention des deux-points uniquement** : on ne flague que les noms de métriques contenant `:` (recording-rule par convention) qu'aucun `record:` du repo ne définit. Les métriques brutes d'exporter (sans `:`) sont laissées à l'heuristique **catalogue statique** (V-later) — sans le `:`, on ne peut plus distinguer « typo de recording rule » de « métrique d'exporter légitime ».
- **Pas de dead-recording-rule en V1** : « recording rule définie mais non référencée intra-repo » ≠ « morte » (consommée par Grafana, un autre repo, la fédération). Savoir si elle est vraiment consommée demande le même contexte externe que le grounding → c'est de la **classe grounding**, pas un check déterministe intra-repo. Faux-positif quasi garanti sinon.

## Garde-fou : index connu-incomplet (interaction avec 0002)

Si des fichiers ont été **skippés** (Helm, 0002) ou ont échoué au parse, l'index repo-wide des `record:` est **connu-incomplet**. Le check missing-ref ne peut alors pas conclure « recording rule inexistante » de façon sûre : une alerting rule (dans un fichier parsable) peut référencer un `record:` défini dans un fichier skippé. Donc **index incomplet ⟹ le Finding missing-ref est downgradé en *advisory*** (jamais Blocking) — sinon l'interaction 0002×0003 fabrique exactement le « check flaky qui bloque » que 0006 interdit.

## Consequences

- Le moteur charge et indexe **tout le repo**, pas seulement le diff — surprenant pour qui suppose qu'un linter de PR ne lit que les fichiers changés.
