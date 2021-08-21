# clipboard-os 

clipboard-os is an online clipboard based on object-storage, now clipboard-os support s3, cos.


## use with docker

```bash
make docker
docker run -p 80:80 -e os_secret=xx:yy -e bucket_url=https://ss.cos.ap-shanghai.myqcloud.com  clipboard
```
