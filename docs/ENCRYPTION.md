# PII Cache PaaS Cryptographic Specification

## Goal

The primary objective is **Cryptographic Zero-Knowledge**: The PII Cache PaaS platform must never hold all the necessary secrets to decrypt client data simultaneously. Decryption must always require a client-managed secret as a mandatory second factor.

## 1. The Core Method: Envelope Encryption with Shared TEK

The system uses **Envelope Encryption**, but the inner key is a **Tenant Encryption Key (TEK)**, which is shared across all records for a single client or project. This simplifies key management.

- **Tenant Encryption Key (TEK)**: A single, cryptographically strong key generated once per organization or project. The TEK is used as a mandatory input for the Final Key Derivation (FKD) process.

- **Key Encryption Key (KEK)**: The Master Key that encrypts (or "wraps") the TEK. The KEK is the most protected secret in the entire system, secured within a dedicated Key Management Service (KMS) or Vault.

## 2. The Three Secrets and Their Custody

Security is enforced by distributing the control of the three essential secrets among the client and the platform.

| Secret Name | Role | Custody | Platform Location |
|-------------|------|---------|-------------------|
| **Tenant Encryption Key (TEK)** | Input for the FKD. Shared per organization. | Platform | Stored in the PII Vault (always Encrypted by the KEK). |
| **Key Encryption Key (KEK)** | Unlocks the TEK. | Platform | Secured inside the FIPS 140-2 certified KMS/Vault. Never leaves this boundary. |
| **Organization Key** | The Failsafe. Second factor for decryption. | Client ONLY. | Never stored in persistent PaaS storage. Only transmitted via secure API call. |

## 3. The Tokenization Process (Ingress)

This sequence details how raw PII is transformed into a non-sensitive Reference Hash (Token) and securely stored.

### Step A: Client Validation & Key Hashing

- **Key Receipt & Hashing**: The API receives the client's raw Organization Key via the `X-Org-Key` header.

- **Verification**: The system calculates a Verifiable Hash of the incoming raw Organization Key. This hash is compared against the known, stored, one-way hash for the organization.

- **Authentication**: If the hashes match, the key is valid. The raw Organization Key is held ephemerally in memory for Step B, then immediately zeroed out.

### Step B: Final Key Derivation for Encryption (FKD)

- **TEK Retrieval**: The system retrieves the client's Encrypted TEK from the PII Vault.

- **TEK Unwrapping**: The Encrypted TEK is sent to the KMS/Vault to be decrypted (unwrapped) using the KEK. The Plaintext TEK is returned.

- **FKD**: The system takes the Plaintext TEK and the raw Organization Key (from Step A) and feeds both into a secure Key Derivation Function (KDF) like HKDF.

- **Final Encryption Key (FEK) Generation**: The KDF fuses the two secrets to produce the Final Encryption Key (FEK).

### Step C: PII Encryption and Persistence

- **IV Generation**: A unique, random Initialization Vector (IV) is generated, as required by AES-256-GCM (Section 5).

- **PII Encryption**: The raw PII is encrypted using the Final Encryption Key (FEK) and the IV:
  ```
  Ciphertext = AES-GCM(PII, FEK, IV)
  ```

- **Token Generation**: A non-sensitive, high-entropy Reference Hash (Token) is created.

- **Synchronous Write-Through**: The complete encrypted bundle (Reference Hash, IV, Ciphertext PII) is written synchronously to the high-speed Redis Cache. The API handler immediately returns the Reference Hash to the client.

- **Asynchronous Commit**: The bundle is pushed to a Message Queue for durable commitment to the PostgreSQL PII Vault by the Persistence Worker.

## 4. The Decryption Process: Final Key Derivation (FKD)

Decryption requires **Two-Factor Authorization** at the cryptographic level.

### Step A: KEK Action (Unwrapping the TEK)

1. The client sends a detokenize request, including the PII Token and their raw Organization Key.

2. The system retrieves the Encrypted TEK from the PII Vault (or Redis).

3. The Encrypted TEK is sent to the KMS/Vault to be unwrapped (decrypted) using the KEK.

4. The KMS returns the Plaintext TEK.

### Step B: Final Key Derivation (FKD)

1. The system takes the Plaintext TEK and the client's raw Organization Key (which sits ephemerally in memory after validation).

2. It feeds both secrets into a highly secure Key Derivation Function (KDF) like HKDF.

3. The KDF fuses the two secrets to produce the **Final Decryption Key (FDK)**.

4. **Failsafe Check**: If the client's Organization Key is wrong, the Final Decryption Key will be completely incorrect, and decryption will fail safely.

### Step C: Final Decryption

- The Final Decryption Key (FDK) is used with AES-256-GCM and the stored IV to decrypt the PII.

- The resulting plaintext PII is returned to the client and immediately zeroed out of the system's memory.

## 5. Cryptographic Assurance: AES-256-GCM

All encryption and decryption of the actual PII data uses the **Advanced Encryption Standard (AES)** with a 256-bit key in **Galois/Counter Mode (GCM)**.

- **Standard**: AES-256 is the current industry standard, highly resistant to known attacks.

- **Integrity**: GCM is an Authenticated Encryption mode, ensuring the data has not been tampered with.

- **Safety Maximization**: A unique, random Initialization Vector (IV) is generated and stored with every piece of PII ciphertext to prevent dangerous IV reuse attacks.

---

## Security Summary

```
┌─────────────────────────────────────────────────────────┐
│           Cryptographic Zero-Knowledge Model            │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Platform Holds:     KEK (in KMS) + Encrypted TEK      │
│  Client Holds:       Organization Key                  │
│                                                         │
│  ✓ Neither party can decrypt alone                     │
│  ✓ Both parties required for decryption                │
│  ✓ Platform never stores Organization Key              │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Key Security Properties

1. **Zero-Knowledge**: Platform cannot decrypt without client's Organization Key
2. **Two-Factor Crypto**: Requires both TEK and Organization Key via KDF
3. **Ephemeral Secrets**: Organization Key never persisted, only in-memory
4. **Authenticated Encryption**: AES-256-GCM provides both confidentiality and integrity
5. **Key Isolation**: KEK never leaves KMS/Vault boundary