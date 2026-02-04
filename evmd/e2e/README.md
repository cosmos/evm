`TODO: Add more information here`

## Keep containers running after tests

If you'd like to keep the docker containers running after the tests complete use the `KEEP_CONTAINERS` environment variable:

```
KEEP_CONTAINERS=true make test-e2e
```

Note that you should manually delete the created containers and networks before running the tests again.
