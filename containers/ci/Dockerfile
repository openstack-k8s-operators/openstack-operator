FROM src
RUN yum update -y && yum install -y python39 python39-pip
RUN rm /usr/bin/python3 && ln -ns /usr/bin/python3.9 /usr/bin/python3
RUN pip3 install pre-commit
