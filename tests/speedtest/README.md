# Speedtest

Speedtest tests the TPS of your application's execution. 

# Build

cd tests/speedtest
go build .

# Run

The default configuration runs with 10k accounts, 4k txs per block, for 100 blocks.

./speedtest

To change parameters run:

./speedtest --acounts 60000 --txs 5000 --blocks 150

IMPORTANT: It is important to ensure your configuration is not exceeding the block max gas/bytes. These are by default 
set to the maximum possible values. However, should you exceed this, the program will report a false value. Use 
--verify-txs to verify that your configuration is sound. Once the configuration is confirmed, run without the flag to 
get true TPS values.