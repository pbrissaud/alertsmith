# Angle mort de conversion assumé : privacy > instrumentation, signal volontairement maigre

0005 (zéro télémétrie / phone-home) + 0008 (aucune vue sur l'usage du grounding ; billing = license, pas metering) laissent le produit **aveugle à sa propre thèse de conversion** : 0012 pose que la conversion repose *entièrement* sur la valeur réelle du 20 % (LLM-jugement-frais + grounding), or aucun feedback loop n'est câblé dessus. Les seuls signaux sont le **renouvellement de license** (laggard) et ce que les users disent volontairement.

## Décision

On **acte l'angle mort comme un trade conscient.** La privacy (« vos règles ne quittent jamais votre runner ») EST le pitch (0005/0007/0008) ; toute instrumentation produit qui donnerait le signal de conversion la contredirait. On choisit la privacy et on **renonce délibérément aux analytics produit**.

Mitigations qui ne cassent PAS la privacy (les seules admises) :
- **Feedback opt-in explicite** — le user *choisit* d'envoyer, jamais par défaut.
- **Proxy de signal via repos publics** — le move go-to-market « scanner les N plus gros rulesets publics » sert aussi de **thermomètre** de la valeur des checks, sans toucher un client.
- **Renouvellement + churn** comme métrique tardive mais honnête.

## Consequences

- Pour un solo qui parie tout sur le 20 %, **zéro analytics produit est un risque réel** — désormais *écrit*, pas subi.
- Toute future tentation d'ajouter de la télémétrie « juste pour comprendre la conversion » doit **rouvrir 0005 explicitement** via un nouvel ADR — ce n'est pas un ajout anodin.
