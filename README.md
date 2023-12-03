# ecs-agent-status
quick script to check the status of the ecs agents for all ecs clusters  where the cluster name containes a given substring

check the agent status for all the clusters containing the string "production"
```bash
ecs-agent-status production
```

The app will print all the agent status values. it wil also exit with errorlevel 1 if any of the agent status values are not ACTIVE
