# Lambdas Salesforce
### Developed in 2018/19

Lambdas that interact with Salesforce API.

To build the lambda into a ready-to-deploy state:
```
$ make all
```

There is 2 lambdas:
- change mail -> will get the list of current users in salesforce database, check their emails and if there is a non-compliance email, and change it to firstname.lastname@enterprise.io.
- sync photo trombi -> will take photos from the internal trombinoscope (wordpress using RDS as database) and upload them to the corresponding user in salesforce.

### Warning: The lambda was developed in 2018, I cannot guarantee the proper functioning.
