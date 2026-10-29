# capcom agent eval

- Date: 2026-10-29
- capcom: capcom 0.12.1
- Agent: `agenteval agent --base-url http://192.168.0.55:1111/v1 --model Qwen3.8-27B-UD-Q4_K_M`

| Task | Result | Calls | Failed | Repeated failures | Time | Why |
| --- | --- | ---: | ---: | ---: | ---: | --- |
| next-claim-close | pass | 5 | 0 | 0 | 4m47.8s |  |
| file-with-dep | pass | 4 | 0 | 0 | 2m3.4s |  |
| find-lock-note | pass | 4 | 1 | 0 | 1m7.3s |  |
| resume-lost-token | pass | 9 | 2 | 0 | 3m32.2s |  |
| find-by-content | pass | 10 | 0 | 0 | 3m31.8s |  |
| context-before-start | pass | 4 | 0 | 0 | 1m37.1s |  |
| blocked-start | pass | 5 | 0 | 0 | 3m31.7s |  |
| move-under-parent | pass | 6 | 0 | 0 | 2m0.2s |  |
| rename-id | pass | 12 | 0 | 0 | 4m21.7s |  |
| add-note | pass | 6 | 0 | 0 | 1m20.8s |  |
| reopen-with-reason | pass | 1 | 0 | 0 | 20.6s |  |
| close-children | pass | 8 | 0 | 0 | 2m18.7s |  |
| **Total** | 12/12 passed | 74 | 3 | 0 | 30m33.2s |  |
