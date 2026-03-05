Requirements:
* python3
* docker
* CUDA (https://developer.nvidia.com/cuda-downloads)

Configure environment for local metaflow runs.

```
python -m venv virtual-env
```

```
source virtual-env/bin/activate
```

```
pip install --require-hashes -r requirements.txt
```


```
metaflow-dev up
```

Use local environment.

```
metaflow-dev shell
source virtual-env/bin/activate
```


Notes:

- If you are having trouble loading large files in minio using the S3 utility, increase resources in the kubernetes deployment for minio

    ```
    kubectl edit deployment minio
    ```


    Change to a value you consider appropiate.

    ```
    resources:
        limits:
        cpu: "1"
        memory: 2Gi
        requests:
        cpu: "1"
        memory: 2Gi
    ```

If using nvidia, remember to install cuda and cuda toolkit.