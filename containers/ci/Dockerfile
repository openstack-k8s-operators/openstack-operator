FROM src
RUN yum update -y && yum install -y python39 python39-pip
RUN alternatives --set python3 /usr/bin/python3.9
RUN pip3 install pre-commit
