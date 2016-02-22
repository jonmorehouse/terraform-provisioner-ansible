#!/usr/bin/env python
#vim:syntax=python

import argparse
import collections
import json
import os
import os.path
import socket
import subprocess
import sys

from ansible.vars import VariableManager
from ansible.parsing.dataloader import DataLoader
from ansible.inventory import Inventory
from ansible.inventory import Host
from ansible.inventory import Group
from ansible.playbook import Playbook
from ansible.executor.task_queue_manager import TaskQueueManager


OptionsClass = collections.namedtuple('Options', ('connection', 'module_path', 'forks', 'remote_user', 'private_key_file',
                                 'ssh_common_args', 'ssh_extra_args', 'sftp_extra_args', 'scp_extra_args',
                                 'become', 'become_method', 'become_user', 'verbosity', 'check'))


def build_inventory(loader, variable_manager, group_names, playbook_basedir):
    inventory = Inventory(loader=loader, variable_manager=variable_manager, host_list=['localhost'])

    # because we just pass in a "host list" which isn't a real inventory file,
    # we explicitly have to add all of the desired groups to the inventory. By
    # default an "all" group is created whenever a new inventory is created
    for group_name in group_names:
        if not inventory.get_group(group_name):
            inventory.add_group(Group(group_name))

    # because we are explicitly adding groups, we also need to make sure that a
    # playbook basedir is set so that `group_vars` can be loaded from the
    # correct directory.
    inventory.set_playbook_basedir(playbook_basedir)
    inventory_hosts = inventory.get_hosts()

    # for each group specified, ensure that the inventory's host (localhost) is
    # explicitly in the group.
    for group_name in group_names:
        group = inventory.get_group(group_name)
        if group.get_hosts():
            continue

        for host in inventory.get_hosts():
            group.add_host(host)

    return inventory


def build_plays(loader, variable_manager, playbook_path, plays=[], hosts=[]):
    playbook = Playbook.load(playbook_path, variable_manager, loader)
    plays = []
    
    for play in playbook.get_plays():
        if play.get_name() in plays:
            plays.append(play)
            continue
        
        if play._ds['hosts'] in hosts:
            plays.append(play)
            continue

        for piece in play._ds['hosts']:
            if piece in hosts:
                plays.append(play)
                break

    return plays


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='ansible-local script for running ansible against localhost')

    parser.add_argument('--playbook', help='full filepath of the playbook', required=True)
    parser.add_argument('--extra-vars', help='json encoded string with extra-var information', required=True, type=lambda x: json.loads(x))
    parser.add_argument('--groups', help='list of groups to apply to the localhost', required=False, type=lambda x: x.split(','))
    parser.add_argument('--plays', help='list of explicitly named plays to run', required=True, type=lambda x: x.split(','))
    parser.add_argument('--hosts', help='list of host groups based plays to run ', required=False, type=lambda x: x.split(','))
    args = parser.parse_args()

    # clean up arguments that were optional or semioptional
    assert args.plays or args.hosts, "either a list of host groups or a list of plays are required"
    if not args.plays:
        args.plays = []
    if not args.hosts:
        args.hosts = []

    playbook_basedir = os.path.dirname(os.path.abspath(args.playbook))
    
    options = OptionsClass(connection='local',
                      module_path=playbook_basedir,
                      forks=100,
                      remote_user=None,
                      private_key_file=None,
                      ssh_common_args=None,
                      ssh_extra_args=None,
                      sftp_extra_args=None,
                      scp_extra_args=None,
                      become=None,
                      become_method='sudo',
                      become_user='root',
                      verbosity=None,
                      check=False)

    loader = DataLoader()
    variable_manager = VariableManager()
    variable_manager.extra_vars = {
        'hosts': 'all',
        'roles': 'all',
    }
    variable_manager.extra_vars.update(args.extra_vars)
    inventory = build_inventory(loader, variable_manager, args.groups, playbook_basedir)

    variable_manager.set_inventory(inventory)
    plays = build_plays(loader, variable_manager, args.playbook, plays=args.plays, hosts=args.hosts)
    
    tqm = TaskQueueManager(
        inventory=inventory,
        variable_manager=variable_manager,
        loader=loader,
        passwords=None,
        options=options,
        stdout_callback='default',
        run_tree=False,
    )

    for play in plays:
        tqm.run(play)
