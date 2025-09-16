# Mocks

The mocks in this folder have been generated using the [mockery](https://vektra.github.io/mockery/latest/) tool.
To regenerate the mocks, run the following commands at the root of this repository:

- `BankKeeper` (reduced interface defined in ERC20 types):

```bash
mockgen -source=./x/erc20/types/interfaces.go -package=mocks -destination=./x/erc20/types/mocks/BankKeeper.go -exclude_interfaces=AccountKeeper,StakingKeeper,EVMKeeper,Erc20Keeper
```

- `EVMKeeper` (reduced interface defined in ERC20 types):

```bash
cd x/erc20/types
mockery --name EVMKeeper
```
