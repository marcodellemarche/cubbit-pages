# Pagina di Login Generata

## Responsabilità
Cubbit Pages genera automaticamente un `index.html` che sostituisce quello
dell'utente durante il deploy cifrato. Quello originale viene cifrato come
`index.html.enc`.

## Struttura HTML generata
- HTML/CSS/JS completamente self-contained (niente dipendenze esterne)
- Design: dark theme, colori brand Cubbit (#009EFF primary, arancione secondario),
  motivo esagonale, font coerenti
- Form con campo password + bottone "Accedi"
- Messaggio di errore per password errata
- Indicatore di caricamento durante la decifratura

## Flusso JS della pagina di login
1. Al caricamento: controlla localStorage (`cubbitseal_password`)
2. Se password presente: redirect immediato alla pagina richiesta (o root)
3. Al submit del form:
   a. Fetcha `_verify.enc`
   b. Tenta decifratura con la password inserita
   c. Se fallisce: mostra errore "Password non corretta"
   d. Se ok: salva in localStorage e redirect
4. La password in localStorage non ha scadenza (rimane fino a logout esplicito)
5. Bottone "Esci" che cancella localStorage e ricarica

## Loader JS per pagine cifrate
Ogni file HTML dell'utente, prima di essere cifrato, viene wrappato:
```html
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Caricamento...</title></head>
<body>
<script>
(function() {
  var pwd = localStorage.getItem('cubbitseal_password');
  if (!pwd) { window.location.href = '/'; return; }
  // fetch del file .enc corrispondente, decifratura, document.write
})();
</script>
</body>
</html>
```

## Template
Il template HTML/JS della pagina di login è in `internal/login/template.html`
come file embedded con `//go:embed`.
