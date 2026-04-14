# Security

If you believe you have found a security vulnerability in Moca, please report it privately
to [security@mocaverse.xyz](mailto:security@mocaverse.xyz).

This document describes how to report vulnerabilities and how the disclosure process
is managed for this repository.

## Guidelines

We require that all researchers:

- Report vulnerabilities privately by email and avoid posting vulnerability information in public places,
  including GitHub, Discord, Telegram, Twitter, or other non-private channels.
- Make every effort to avoid privacy violations, degradation of user experience, disruption to production systems,
  and destruction of data.
- Keep any information about vulnerabilities that you discover confidential between yourself and the engineering team
  until the issue has been resolved and disclosed.
- Avoid posting personally identifiable information, privately or publicly.

If you follow these guidelines when reporting an issue to us, we commit to:

- Not pursue or support legal action related to your good-faith security research on the reported vulnerability.
- Work with you to understand, resolve, and ultimately disclose the issue in a timely fashion.

## Disclosure Process

Moca uses the following disclosure process:

1. Once a security report is received by email, the team works to verify the issue
   and confirm its severity level using [CVSS](https://nvd.nist.gov/vuln-metrics/cvss)
   and internal triage criteria.

    1. Two people from the affected project will review, replicate, and acknowledge the report
       within 48-96 hours of the alert according to the table below:

        | Security Level       | Hours to First Response (ACK) from Escalation |
        | -------------------- | --------------------------------------------- |
        | Critical             | 48                                            |
        | High                 | 96                                            |
        | Medium               | 96                                            |
        | Low or Informational | 96                                            |
        | None                 | 96                                            |

    2. If the report is not applicable or reproducible,
       the security contact will revert to the reporter to request more information or close the report.
    3. The report is confirmed to the reporter once the issue has been validated.

2. The team determines the vulnerability's potential impact on Moca.

    1. Vulnerabilities with `Informational` and `Low` categorization may result in a public issue after remediation.
    2. Vulnerabilities with `Medium` categorization result in the creation of an internal ticket and a code patch.
    3. Vulnerabilities with `High` or `Critical` categorization result in the
       [creation of a new Security Advisory](https://docs.github.com/en/code-security/repository-security-advisories/creating-a-repository-security-advisory).

Once the vulnerability severity is defined, the following steps apply:

- For `High` and `Critical`:
    1. Patches are prepared for supported releases of Moca in a
       [temporary private fork](https://docs.github.com/en/code-security/repository-security-advisories/collaborating-in-a-temporary-private-fork-to-resolve-a-repository-security-vulnerability)
       of the repository.
    2. Only relevant parties will be notified about an upcoming upgrade,
       including validators, core maintainers, and users directly affected by the vulnerability.
    3. Twenty-four hours following this notification, relevant releases with the patch will be made public.
    4. Node operators and validators update to the patched releases.
    5. A week, or less, after the security vulnerability has been patched on Moca,
       we will disclose that the relevant release contained a security fix.
    6. After an additional two weeks, we will publish a public announcement of the vulnerability.
       We will also publish a GitHub Security Advisory and a
       [CVE](https://en.wikipedia.org/wiki/Common_Vulnerabilities_and_Exposures) when appropriate.

- For `Informational`, `Low`, and `Medium` severities:
    1. `Medium` and `Low` severity reports are tracked privately until the fix is ready
       and are then disclosed through the normal issue or release process.
       `Informational` reports may be tracked and prioritized case by case.
    2. After the fix is released, we may publish additional details about the vulnerability and the response.

This process can take time.
Every effort will be made to handle reports as quickly as possible.
However, it is important that we follow the process described above
to ensure that disclosures are handled consistently
and to keep Moca and its downstream dependent projects as secure as possible.

### Reward Policy

Moca does not currently operate a public bug bounty program.
Reports are reviewed privately, and any reward decision, if applicable,
is handled on a case-by-case basis.

### Contact

If you need to reach out to the team directly,
please email [security@mocaverse.xyz](mailto:security@mocaverse.xyz).
