
tmpl = open('crushtest.route.template.yaml').read()

for i in range(100):
    print(tmpl.replace('{{svc_name}}', f'inventory-{i+1}'))
