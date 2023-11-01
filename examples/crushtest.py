
tmpl = open('crushtest.route.template.yaml').read()

for i in range(10):
    print(tmpl.replace('{{svc_name}}', f'inventory-{i+1}'))
