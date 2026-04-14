# Decisioni architetturali

## Flusso deploy in chiaro
1. CLI legge credenziali (flag > env > config file)
2. Visita ricorsiva della cartella sorgente
3. Per ogni file: PutObject su Cubbit
4. Se modalità ACL per file: PutObjectAcl public-read su ogni file
5. Se modalità bucket policy: mostra snippet da applicare una volta sola
6. Output: URL del sito deployato

## Flusso deploy cifrato
1. Come sopra, ma ogni file (tranne index.html) viene cifrato con AES-256-GCM
2. I file cifrati vengono caricati con estensione `.enc` (es. `about.html` → `about.html.enc`)
3. Viene generato un `index.html` di login che sostituisce quello dell'utente
   (quello originale viene cifrato come `index.html.enc`)
4. Ogni pagina cifrata contiene uno snippet JS inline che:
   - All'avvio legge la password da localStorage (`cubbitseal_password`)
   - Se assente: redirect a `index.html`
   - Se presente: fetcha il file `.enc`, decifra in memoria, renderizza il contenuto

## Pagina di login generata
- È un file HTML standalone, completamente self-contained (CSS e JS inline)
- Contiene il form per inserire la password
- Al submit: tenta di decifrare un file "canary" (piccolo file `.enc` dedicato)
  per verificare che la password sia corretta prima di salvarla in localStorage
- Se corretta: salva in `localStorage.cubbitseal_password` e redirect alla pagina originale
- Il "canary" è un file `_verify.enc` caricato insieme al sito, che contiene
  una stringa nota cifrata — permette di validare la password senza scaricare
  file grandi

## Navigazione su sito cifrato
- Ogni file HTML cifrato, quando viene servito, deve prima essere decifrato
- Il browser non può eseguire HTML cifrato direttamente
- Soluzione: ogni file `.enc` è scaricato via fetch() dalla pagina di login
  o da un piccolo "loader" JS, decifrato in memoria, e il risultato viene
  scritto nel DOM con `document.open()/write()/close()`
- Questo significa che l'URL nel browser rimane `index.html` durante la navigazione
  (limitazione accettabile per v1)

## Gestione ACL vs Bucket Policy
Due modalità per rendere i file pubblicamente leggibili:

### Modalità ACL per file (default)
- Per ogni PutObject, viene chiamato anche PutObjectAcl con `public-read`
- Pro: nessuna configurazione bucket richiesta
- Contro: richiede permesso `s3:PutObjectAcl` nelle credenziali
- Contro: più lento (doppia chiamata per ogni file)

### Modalità bucket policy (consigliata)
- L'utente applica una bucket policy che concede GetObject a tutti
- CLI mostra lo snippet da applicare una sola volta
- Pro: più veloce, meno permessi richiesti nelle credenziali
- Pro: approccio standard per hosting di siti statici
- Attivata con flag `--public-bucket`

## Distribuzione CLI
- Binari compilati per: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- Hostati su bucket Cubbit pubblico: `https://s3.cubbit.eu/cubbit-pages-releases/`
- Script install.sh rileva OS/arch e scarica il binario corretto
- Versioning semantico, il binario include la versione (`cubbit-pages version`)

## Lifecycle policy
Mai applicata automaticamente. La CLI mostra uno snippet `aws s3api` con
warning esplicito se il bucket è dedicato esclusivamente a Cubbit Pages.
