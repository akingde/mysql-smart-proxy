sysbench --test=oltp --mysql-host=10.128.161.143 --mysql-port=9696 --mysql-user=root --mysql-password=Initial0 --mysql-db=CSM --oltp-table-size=500000 --max-requests=0 --oltp-test-mode=nontrx --oltp-nontrx-mode=select --oltp-read-only=off --max-time=120 --num-threads=128 run
#./sysbench --test=./tests/db/select.lua --mysql-host=10.128.161.143 --mysql-port=9696 --mysql-user=root --mysql-password=Initial0 --mysql-db=CSM --oltp-test-mode=complex --oltp-read-only=off --report-interval=5 --report-checkpoints=15 --tx-rate=24 --num-threads=128 run